package controllers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/go-logr/logr"
	"github.com/hashicorp/vault-client-go"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ExternalHandler handles external vaults
type ExternalHandler struct {
	client.Client
	Scheme     *runtime.Scheme
	startedMux sync.Mutex
	started    bool
	secrets    []corev1.Secret
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalHandler) SetupWithManager(mgr ctrl.Manager, secretsExternal []corev1.Secret) error {
	r.secrets = secretsExternal
	return mgr.Add(r)
}

func (r *ExternalHandler) Start(ctx context.Context) error {
	r.startedMux.Lock()
	if r.started {
		return errors.New("handler is already running")
	}
	r.started = true
	r.startedMux.Unlock()

	grp, ctx := errgroup.WithContext(ctx)

	for _, s := range r.secrets {
		grp.Go(func() error {
			err := r.setupVaultCheckLoop(ctx, s)
			if err != nil {
				return err
			}
			return context.Canceled
		})

	}

	_ = grp.Wait()
	return nil
}

func (r *ExternalHandler) setupVaultCheckLoop(ctx context.Context, secret corev1.Secret) error {
	logger := log.FromContext(ctx).WithValues("secret", secret.Name)

	duration, err := r.getInterval(secret, logger)
	if err != nil {
		return err
	}

	srcCl, err := r.getSourceClient(secret)
	if err != nil {
		return err
	}

	trgtsCl, err := r.getTargetClients(secret, srcCl)
	if err != nil {
		return err
	}

	t := time.NewTicker(duration).C
	for {
		r.executeCheck(logger, srcCl, trgtsCl)
		select {
		case <-t:
			continue
		case <-ctx.Done():
			return nil
		}
	}
}

func (r *ExternalHandler) executeCheck(logger logr.Logger, srcCl *vault.Client, trgtCl []*vault.Client) {
	logger.Info("starting seal check")

	logger.Info("seal check completed")
}

func (r *ExternalHandler) getInterval(secret corev1.Secret, logger logr.Logger) (time.Duration, error) {
	str := secret.Labels[constants.LabelExternal]
	duration, err := time.ParseDuration(str)
	if err != nil {
		logger.Error(err, "interval parsing failed, using default", "invalid", str, "actual", constants.DefaultExternalInterval)
		duration = constants.DefaultExternalInterval
	}
	return duration, err
}

func (r *ExternalHandler) getSourceClient(secret corev1.Secret) (*vault.Client, error) {
	src, ok := secret.Labels[constants.LabelExternalSource]
	if !ok {
		return nil, errors.New("no source found")
	}

	return newClient(src, false)
}

func (r *ExternalHandler) getTargetClients(secret corev1.Secret, srcCl *vault.Client) ([]*vault.Client, error) {

	trgt, ok := secret.Labels[constants.LabelExternalTargets]
	if !ok {
		return nil, errors.New("no targets found")
	}

	trgts := strings.Split(trgt, ";")
	trgtsCl := make([]*vault.Client, len(trgts))

	if len(trgts) == 1 && trgts[0] == srcCl.Configuration().Address {
		trgtsCl = append(trgtsCl, srcCl)
	} else {
		for _, t := range trgts {
			tcl, err := newClient(t, false)
			if err != nil {
				return nil, err
			}
			trgtsCl = append(trgtsCl, tcl)
		}
	}

	return trgtsCl, nil
}
