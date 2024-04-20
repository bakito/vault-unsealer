package logging

import (
	"os"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// PrepareLogger prepare logging configuration for controller runtime
func PrepareLogger(json bool) {
	ctrl.SetLogger(NewLogger(json))
	// replace klog logger
	klog.SetLogger(ctrl.Log)
}

// NewLogger create a new logger
func NewLogger(json bool) logr.Logger {
	// Use json encoder with iso timestamps
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	opts := crzap.Options{
		Encoder: zapcore.NewJSONEncoder(encCfg),
	}

	if !json {
		opts.Encoder = zapcore.NewConsoleEncoder(encCfg)
	}
	if ll, ok := os.LookupEnv(constants.EnvLogLevel); ok {
		l, err := zapcore.ParseLevel(ll)
		if err != nil {
			opts.Level = l
		}
	}
	return crzap.New(crzap.UseFlagOptions(&opts))
}
