package obfuscator

import "testing"

func TestURLUserinfoObfuscator(t *testing.T) {
	o := NewURLUserinfoObfuscator()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "http proxy with credentials",
			input: "httpProxy: http://proxy-user:secret@proxy.example.com:3128",
			want:  "httpProxy: http://proxy-user:****@proxy.example.com:3128",
		},
		{
			name:  "https proxy with credentials",
			input: "httpsProxy: https://proxy-user:secret@proxy.example.com:3128",
			want:  "httpsProxy: https://proxy-user:****@proxy.example.com:3128",
		},
		{
			name:  "url without credentials unchanged",
			input: "httpProxy: http://proxy.example.com:3128",
			want:  "httpProxy: http://proxy.example.com:3128",
		},
		{
			name:  "multiple proxy fields",
			input: "http://a:p1@h1:1 https://b:p2@h2:2",
			want:  "http://a:****@h1:1 https://b:****@h2:2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := o.Contents(tc.input); got != tc.want {
				t.Fatalf("Contents(%q) = %q, want %q", tc.input, got, tc.want)
			}
			if got := o.Path(tc.input); got != tc.want {
				t.Fatalf("Path(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
