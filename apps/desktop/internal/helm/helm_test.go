package helm

import "testing"

func TestCheckRepo(t *testing.T) {
	ok := [][2]string{{"https://charts.example.com", "nginx"}, {"", "oci://registry.example.com/charts/app"}, {"", "./local-chart"}, {"", "nginx"}}
	for _, c := range ok {
		if err := checkRepo(c[0], c[1]); err != nil {
			t.Errorf("%v rejected: %v", c, err)
		}
	}
	bad := [][2]string{{"http://charts.example.com", "nginx"}, {"", "http://evil.example.com/app.tgz"}}
	for _, c := range bad {
		if err := checkRepo(c[0], c[1]); err == nil {
			t.Errorf("%v accepted", c)
		}
	}
	if v, err := parseValues(""); err != nil || v == nil {
		t.Error("empty values must be an empty map")
	}
}
