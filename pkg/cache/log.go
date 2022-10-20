package cache

import "strings"

type logrWriter struct{}

func (w *logrWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSuffix(string(p), "\n")
	if strings.Contains(msg, "[DEBUG] ") {
		serfLog.V(2).Info(strings.Split(msg, "[DEBUG] ")[1])
	} else if strings.Contains(msg, "[INFO] ") {
		serfLog.Info(strings.Split(msg, "[INFO] ")[1])
	} else if strings.Contains(msg, "[ERROR] ") {
		serfLog.Error(nil, strings.Split(msg, "[ERROR] ")[1])
	} else if strings.Contains(msg, "[WARN] ") {
		serfLog.Error(nil, strings.Split(msg, "[WARN] ")[1])
	}
	return 0, nil
}
