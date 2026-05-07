package adapter

import (
	"testing"
)

func TestRegistryRegisterResolve(t *testing.T) {
	r := NewRegistry()
	r.Register(ProcessSpec{
		Language:   "go",
		AdapterID:  "delve",
		DAPType:    "go",
		Binary:     "/bin/ls",
		BinaryArgs: []string{"dap"},
		ExtraPath:  []string{"/go/bin"},
	})

	spec, err := r.Resolve("go")
	if err != nil {
		t.Fatalf("Resolve(go): %v", err)
	}
	if spec.AdapterID != "delve" {
		t.Errorf("AdapterID = %q, want %q", spec.AdapterID, "delve")
	}
	if spec.DAPType != "go" {
		t.Errorf("DAPType = %q, want %q", spec.DAPType, "go")
	}
	if spec.Binary != "/bin/ls" {
		t.Errorf("Binary = %q, want %q", spec.Binary, "/bin/ls")
	}
}

func TestRegistryResolveEmptyDefaultsToGo(t *testing.T) {
	r := NewRegistry()
	r.Register(ProcessSpec{
		Language:   "go",
		AdapterID:  "delve",
		DAPType:    "go",
		Binary:     "/bin/ls",
		BinaryArgs: []string{"dap"},
	})

	spec, err := r.Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\") should default to go: %v", err)
	}
	if spec.AdapterID != "delve" {
		t.Errorf("default language resolved wrong adapter: %q", spec.AdapterID)
	}
}

func TestRegistryResolveUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Resolve("python")
	if err == nil {
		t.Fatal("expected error for unknown language, got nil")
	}
}

func TestRegistryResolveMissingBinary(t *testing.T) {
	r := NewRegistry()
	r.Register(ProcessSpec{
		Language:  "python",
		AdapterID: "debugpy",
		DAPType:   "python",
		Binary:    "",
	})

	_, err := r.Resolve("python")
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

func TestRegistrySetBinary(t *testing.T) {
	r := NewRegistry()
	r.Register(ProcessSpec{
		Language:  "go",
		AdapterID: "delve",
		DAPType:   "go",
		Binary:    "",
	})

	// Should fail before setting
	_, err := r.Resolve("go")
	if err == nil {
		t.Fatal("expected error before SetBinary")
	}

	// Set custom path to a real executable
	if err := r.SetBinary("go", "/bin/ls"); err != nil {
		t.Fatalf("SetBinary: %v", err)
	}

	spec, err := r.Resolve("go")
	if err != nil {
		t.Fatalf("Resolve after SetBinary: %v", err)
	}
	if spec.Binary != "/bin/ls" {
		t.Errorf("Binary = %q, want /bin/ls", spec.Binary)
	}
}

func TestRegistrySetBinaryUnknown(t *testing.T) {
	r := NewRegistry()
	err := r.SetBinary("python", "/usr/bin/debugpy")
	if err == nil {
		t.Fatal("expected error for unknown language")
	}
}

func TestRegisterPanicsOnEmptyLanguage(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty language")
		}
	}()
	r := NewRegistry()
	r.Register(ProcessSpec{AdapterID: "delve"})
}

func TestRegistryAll(t *testing.T) {
	r := NewRegistry()
	r.Register(ProcessSpec{Language: "go", AdapterID: "delve", Binary: "/bin/dlv"})
	r.Register(ProcessSpec{Language: "python", AdapterID: "debugpy", Binary: "/bin/debugpy"})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d specs, want 2", len(all))
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	r.Register(ProcessSpec{
		Language: "go",
		Label:    "Go (Delve)",
		Binary:   "/bin/dlv",
	})

	spec, ok := r.Get("go")
	if !ok {
		t.Fatal("Get(go) returned false")
	}
	if spec.Label != "Go (Delve)" {
		t.Errorf("Label = %q", spec.Label)
	}

	_, ok = r.Get("python")
	if ok {
		t.Fatal("Get(python) should return false")
	}
}

func TestRegistryInstalled(t *testing.T) {
	r := NewRegistry()
	r.Register(ProcessSpec{
		Language:   "go",
		Binary:     "",
		BinaryName: "dlv",
	})

	if r.Installed("go") {
		t.Fatal("Installed should be false when Binary is empty")
	}

	r.SetBinary("go", "/bin/ls") // ls exists everywhere
	if !r.Installed("go") {
		t.Fatal("Installed should be true for /bin/ls")
	}

	if r.Installed("python") {
		t.Fatal("Installed should be false for unknown language")
	}
}

func TestRegistryRediscover(t *testing.T) {
	r := NewRegistry()
	r.Register(ProcessSpec{
		Language:   "test",
		BinaryName: "ls", // always on PATH
		Binary:     "",
		ExtraPath:  nil,
	})

	r.Rediscover("test")
	if !r.Installed("test") {
		t.Fatal("Rediscover should find 'ls' on PATH")
	}
}

func TestFindBinary(t *testing.T) {
	// ls should always be on PATH
	p := FindBinary("ls", nil)
	if p == "" {
		t.Fatal("FindBinary(ls) should find something")
	}

	// nonexistent binary
	p = FindBinary("nonexistent-adapter-xyz", nil)
	if p != "" {
		t.Fatalf("FindBinary(nonexistent) should return empty, got %q", p)
	}

	// Check extra paths work
	p = FindBinary("ls", []string{"/bin"})
	if p == "" {
		t.Fatal("FindBinary(ls, [/bin]) should find something")
	}
}
