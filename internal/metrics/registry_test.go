package metrics

import (
	"testing"

	"github.com/netikras/procfit/internal/model"
)

func TestDefaultRegistry_UniqueIDsAndAliases(t *testing.T) {
	// NewDefault MustRegisters; a duplicate id/alias would panic. Reaching here
	// proves uniqueness. Also verify alias resolution.
	r := NewDefault()
	if !r.Has("cpu") {
		t.Fatal("cpu missing from default registry")
	}
	id, ok := r.ResolveID("disk-read-bytes")
	if !ok || id != model.MetricID("disk-rbps") {
		t.Fatalf("alias disk-read-bytes should resolve to disk-rbps, got %q ok=%v", id, ok)
	}
}

func TestRegistry_RegisterDuplicates(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(Descriptor{ID: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Descriptor{ID: "x"}); err == nil {
		t.Fatal("duplicate id should error")
	}
	if err := r.Register(Descriptor{ID: "y", Aliases: []string{"x"}}); err == nil {
		t.Fatal("alias colliding with existing id should error")
	}
	if err := r.Register(Descriptor{ID: "z", Aliases: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(Descriptor{ID: "w", Aliases: []string{"a"}}); err == nil {
		t.Fatal("duplicate alias should error")
	}
	if err := r.Register(Descriptor{ID: ""}); err == nil {
		t.Fatal("empty id should error")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewDefault()
	d, ok := r.Get("disk-wbps")
	if !ok || !d.IsRate() {
		t.Fatalf("disk-wbps should be a rate metric")
	}
	if _, ok := r.Get("nope"); ok {
		t.Fatal("unknown metric should not be found")
	}
	rss, _ := r.Get("rss")
	if rss.IsRate() {
		t.Fatal("rss is a gauge, not a rate")
	}
	if rss.Aggregation != AggSumOncePerProc {
		t.Fatal("rss must aggregate sum-once-per-process to avoid double counting")
	}
}

func TestProfileResolve(t *testing.T) {
	r := NewDefault()
	none, _ := r.Resolve(ProfileNone)
	if len(none) != 0 {
		t.Fatal("none profile should be empty")
	}
	all, _ := r.Resolve(ProfileAll)
	if len(all) != len(r.All()) {
		t.Fatal("all profile should include every registered metric")
	}
	light, _ := r.Resolve(ProfileLight)
	if len(light) == 0 {
		t.Fatal("light profile should be non-empty")
	}
	if _, err := r.Resolve(ProfileName("bogus")); err == nil {
		t.Fatal("unknown profile should error")
	}
	io, _ := r.Resolve(ProfileIO)
	if len(io) == 0 {
		t.Fatal("io profile should include existing io metrics")
	}
	// battery: a known profile that always yields at least the no-root signals
	// (cpu / cpu-normalized / ctxsw-voluntary), regardless of eBPF availability.
	if !IsKnownProfile("battery") {
		t.Fatal("battery must be a known profile")
	}
	bat, _ := r.Resolve(ProfileBattery)
	has := func(id model.MetricID) bool {
		for _, m := range bat {
			if m == id {
				return true
			}
		}
		return false
	}
	if !has("cpu") || !has("cpu-normalized") || !has("ctxsw-voluntary") {
		t.Fatalf("battery profile missing the no-root signals: %v", bat)
	}
}

func TestKnownProfilesMatchesSpecs(t *testing.T) {
	// The display list (KnownProfiles / ProfileNames) must stay in sync with the
	// actual profileSpecs, so help text can never drift from what resolves.
	derived := map[ProfileName]bool{ProfileNone: true, ProfileLight: true, ProfileAll: true}
	inKnown := map[ProfileName]bool{}
	for _, p := range KnownProfiles {
		inKnown[p] = true
		if !derived[p] {
			if _, ok := profileSpecs[p]; !ok {
				t.Fatalf("KnownProfiles lists %q but there is no profileSpecs entry", p)
			}
		}
	}
	for p := range profileSpecs {
		if !inKnown[p] {
			t.Fatalf("profileSpecs has %q missing from KnownProfiles (help list would omit it)", p)
		}
		if !IsKnownProfile(string(p)) {
			t.Fatalf("IsKnownProfile should recognize spec profile %q", p)
		}
	}
	if len(ProfileNames()) != len(KnownProfiles) {
		t.Fatal("ProfileNames must mirror KnownProfiles")
	}
}

func TestResolveSelection(t *testing.T) {
	r := NewDefault()
	// none + explicit adds
	got, err := r.ResolveSelection(ProfileNone, []Override{{Add: true, ID: "cpu"}, {Add: true, ID: "rss"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "cpu" || got[1] != "rss" {
		t.Fatalf("selection order wrong: %v", got)
	}
	// light minus one, plus alias
	got, err = r.ResolveSelection(ProfileLight, []Override{{Add: false, ID: "vsz"}, {Add: true, ID: "disk-read-bytes"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range got {
		if id == "vsz" {
			t.Fatal("vsz should have been removed")
		}
	}
	// unknown metric errors
	if _, err := r.ResolveSelection(ProfileNone, []Override{{Add: true, ID: "bogus"}}); err == nil {
		t.Fatal("unknown metric in selection should error")
	}
}

func TestIsKnownProfile(t *testing.T) {
	if !IsKnownProfile("light") || IsKnownProfile("nope") {
		t.Fatal("IsKnownProfile wrong")
	}
}
