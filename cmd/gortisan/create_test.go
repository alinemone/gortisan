package main

import "testing"

func TestCreateTarget(t *testing.T) {
	cases := []struct {
		args         []string
		flag         string
		name, module string
		wantErr      bool
	}{
		{[]string{"blog"}, "", "blog", "", false},
		{[]string{"blog", "github.com/you/blog"}, "", "blog", "github.com/you/blog", false},
		{[]string{"github.com/you/blog"}, "", "blog", "github.com/you/blog", false},
		{[]string{"blog"}, "github.com/you/blog", "blog", "github.com/you/blog", false},
		{[]string{"blog", "github.com/you/blog"}, "github.com/you/blog", "blog", "github.com/you/blog", false},
		{[]string{"blog", "github.com/you/blog"}, "github.com/you/other", "", "", true},
		{nil, "", "", "", true},
		{[]string{"a", "b", "c"}, "", "", "", true},
	}
	for _, c := range cases {
		name, module, err := createTarget(c.args, c.flag)
		if (err != nil) != c.wantErr || name != c.name || module != c.module {
			t.Errorf("createTarget(%q, %q) = %q, %q, %v", c.args, c.flag, name, module, err)
		}
	}
}
