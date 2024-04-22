package logging

import (
	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// SetupLogger configures logging for the controller runtime.
func SetupLogger(json bool) {
	ctrl.SetLogger(newLogger(json))
	// Replace klog logger with controller-runtime logger
	klog.SetLogger(ctrl.Log)
}

// newLogger creates a new logger based on the provided configuration.
func newLogger(json bool) logr.Logger {
	// Use JSON encoder with ISO timestamps by default
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// Use console encoder if JSON format is not requested
	encoder := zapcore.NewJSONEncoder(encCfg)
	if !json {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	}

	opts := crzap.Options{
		Encoder: encoder,
	}
	return crzap.New(crzap.UseFlagOptions(&opts))
}
