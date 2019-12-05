package peer

import (
	"fmt"
	"github.com/pion/logging"
	"github.com/rs/zerolog/log"
)

type loggerFactory struct {
}

func (lf *loggerFactory) NewLogger(scope string) logging.LeveledLogger {
	return &logger{scope: scope}
}

type logger struct {
	scope string
}

func (l *logger) Trace(msg string) {
	log.Trace().Msgf("[%s] %s", l.scope, msg)
}

func (l *logger) Tracef(format string, args ...interface{}) {
	log.Trace().Msgf("[%s] %s", l.scope, fmt.Sprintf(format, args...))
}

func (l *logger) Debug(msg string) {
	log.Debug().Msgf("[%s] %s", l.scope, msg)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	log.Debug().Msgf("[%s] %s", l.scope, fmt.Sprintf(format, args...))
}

func (l *logger) Info(msg string) {
	log.Info().Msgf("[%s] %s", l.scope, msg)
}

func (l *logger) Infof(format string, args ...interface{}) {
	log.Info().Msgf("[%s] %s", l.scope, fmt.Sprintf(format, args...))
}

func (l *logger) Warn(msg string) {
	log.Warn().Msgf("[%s] %s", l.scope, msg)
}

func (l *logger) Warnf(format string, args ...interface{}) {
	log.Warn().Msgf("[%s] %s", l.scope, fmt.Sprintf(format, args...))
}

func (l *logger) Error(msg string) {
	log.Error().Msgf("[%s] %s", l.scope, msg)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	log.Error().Msgf("[%s] %s", l.scope, fmt.Sprintf(format, args...))
}
