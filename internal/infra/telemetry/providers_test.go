package telemetry

import "testing"

func TestNewSampler(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		"always_on",
		"always_off",
		"parentbased_always_on",
		"parentbased_always_off",
	} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			if _, err := newSampler(value); err != nil {
				t.Fatalf("newSampler(%q): %v", value, err)
			}
		})
	}
	if _, err := newSampler("unknown"); err == nil {
		t.Fatal("expected unsupported sampler error")
	}
}
