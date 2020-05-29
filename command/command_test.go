package command

import "testing"

var (
	goodUserNames = []string{
		"bar",
		"foo-bar",
		"foo-bar-baz",
		"foo1",
		"foo-1",
		"foo-1-bar",
		"foo.bar",
		"foo_bar",
		"foo.bar_hey",
		"foo_bar.hello",
		"f12oo-bar33",
	}
	badUserNames = []string{
		"-bar",
		"bar-",
		"-foo-bar",
		"foo-bar-",
		"foo--bar",
		"foo#bar",
		"1foobar",
		"1foobar",
		"_foobar",
		".foobar",
	}
)

func TestIsUserNameValidFormat(t *testing.T) {
	for _, username := range goodUserNames {
		ok := isUserNameValidFormat(username)
		if !ok {
			t.Errorf("expect valid user for %q", username)
		}
	}
	for _, username := range badUserNames {
		ok := isUserNameValidFormat(username)
		if ok {
			t.Errorf("expect invalid user for %q", username)
		}
	}
}
