package keys

import "testing"

func TestParseNotation(t *testing.T) {
	ks, err := Parse("3dw<Esc><C-r>x<M-a><CR>")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"3", "d", "w", "<Esc>", "<C-r>", "x", "<M-a>", "<CR>"}
	if len(ks) != len(want) {
		t.Fatalf("got %d keys", len(ks))
	}
	for i, k := range ks {
		if k.String() != want[i] {
			t.Errorf("key %d = %q want %q", i, k.String(), want[i])
		}
	}
}

func TestParseSpacesAndAngles(t *testing.T) {
	ks, err := Parse("a <Space> <lt> iw")
	if err != nil {
		t.Fatal(err)
	}
	if len(ks) != 8 {
		t.Fatalf("got %d keys: %v", len(ks), ks)
	}
	if !ks[2].IsRune() || ks[2].R != ' ' {
		t.Fatalf("space: %v", ks[2])
	}
	if !ks[4].IsRune() || ks[4].R != '<' {
		t.Fatalf("lt: %v", ks[4])
	}
}

func TestCtrlKeys(t *testing.T) {
	ks := MustParse("<C-c><C-[>")
	if !ks[0].IsCtrl() || ks[0].R != 'C' {
		t.Fatalf("c-c: %v", ks[0])
	}
	if ks[0].String() != "<C-c>" {
		t.Fatalf("string: %q", ks[0].String())
	}
}
