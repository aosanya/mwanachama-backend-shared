package identity

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidRaw = errors.New("identity: invalid raw")

var SegmentPattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

type Raw struct {
	Module   string
	Instance string
	Identity string
}

func New(module, instance, identity string) (Raw, error) {
	r := Raw{Module: module, Instance: instance, Identity: identity}
	if err := r.check(); err != nil {
		return Raw{}, err
	}
	return r, nil
}

func Parse(raw string) (Raw, error) {
	parts := strings.SplitN(raw, "/", 3)
	if len(parts) != 3 {
		return Raw{}, fmt.Errorf("%w: %q is not <module>/<instance>/<identity>", ErrInvalidRaw, raw)
	}
	r := Raw{Module: parts[0], Instance: parts[1], Identity: parts[2]}
	if err := r.check(); err != nil {
		return Raw{}, err
	}
	return r, nil
}

func (r Raw) check() error {
	for _, segment := range []string{r.Module, r.Instance} {
		if !SegmentPattern.MatchString(segment) {
			return fmt.Errorf("%w: segment %q of %q must match %s", ErrInvalidRaw, segment, r.String(), SegmentPattern)
		}
	}
	if err := checkIdentity(r.Identity); err != nil {
		return fmt.Errorf("%w: %q: %v", ErrInvalidRaw, r.String(), err)
	}
	return nil
}

func checkIdentity(identity string) error {
	if identity == "" {
		return errors.New("the identity is empty")
	}
	if !utf8.ValidString(identity) {
		return errors.New("the identity is not UTF-8")
	}
	if strings.TrimSpace(identity) != identity {
		return errors.New("the identity starts or ends with whitespace")
	}
	for _, c := range identity {
		if unicode.IsControl(c) {
			return fmt.Errorf("the identity carries the control character %U", c)
		}
		if unicode.In(c, unicode.Cf, unicode.Zl, unicode.Zp) {
			return fmt.Errorf("the identity carries the invisible or line-breaking character %U", c)
		}
	}
	return nil
}

func (r Raw) String() string {
	return r.Module + "/" + r.Instance + "/" + r.Identity
}

func (r Raw) Key() string {
	return KeyOf(r.String())
}
