package log

import (
	"errors"

	authnLog "github.com/cyberark/conjur-authn-k8s-client/pkg/log"
)

func LogErrorsAndInfos(errs []error, infos []error) error {
	for _, info := range infos {
		authnLog.Info(info.Error())
	}

	if len(errs) > 0 {
		for _, err := range errs {
			authnLog.Error(err.Error())
		}
		return errors.New("fatal errors occurred, check Secrets Provider logs")
	}
	return nil
}
