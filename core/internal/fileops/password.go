package fileops

import (
	"context"
	"errors"
	"slices"
)

var errPasswordNeeded = errors.New("archive needs a password")

type badPasswordError struct{ err error }

func (e *badPasswordError) Error() string { return e.err.Error() }

func (e *badPasswordError) Unwrap() error { return e.err }

func isBadPassword(err error) bool {
	var bad *badPasswordError
	return errors.As(err, &bad)
}

func (r *runner) knownPasswords(archive string) []string {
	var known []string
	for _, candidate := range []string{r.job.spec.password, r.password, r.job.mgr.archives.password(archive)} {
		if candidate != "" && !slices.Contains(known, candidate) {
			known = append(known, candidate)
		}
	}
	return known
}

func (r *runner) unlock(archive string, attempt func(password string) error) error {
	known := r.knownPasswords(archive)
	password := ""
	typed := false
	for {
		err := attempt(password)
		switch {
		case err == nil:
			if password != "" {
				r.password = password
				r.job.mgr.archives.keepPassword(archive, password)
			}
			return nil
		case errors.Is(err, context.Canceled):
			return err
		case !errors.Is(err, errPasswordNeeded) && !isBadPassword(err):
			return err
		}
		if len(known) > 0 {
			password, known = known[0], known[1:]
			continue
		}
		wrong := typed && isBadPassword(err)
		password, err = r.askPassword(archive, wrong)
		if err != nil {
			return err
		}
		typed = true
	}
}

func (r *runner) askPassword(archive string, wrong bool) (string, error) {
	prompt := &PasswordPrompt{
		ID:      r.job.nextID("p"),
		Path:    archive,
		Wrong:   wrong,
		Actions: []Action{ActionUnlock, ActionSkip, ActionCancel},
	}
	answer := r.job.ask(event{Kind: "password", JobID: r.job.id, PasswordPrompt: prompt}, StatePausedPassword)
	switch answer.Action {
	case ActionUnlock:
		return answer.Password, nil
	case ActionSkip:
		r.skip()
		return "", errSkipped
	default:
		return "", context.Canceled
	}
}
