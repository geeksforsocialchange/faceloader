package main

import "testing"

func TestRemoveDuplicate(t *testing.T) {
	var names []string

	ans := removeDuplicateStr(names)
	if len(ans) != 0 {
		t.Errorf("removeDuplicateStr([]) = %v; want []", ans)
	}

	names = append(names, "Alice")
	ans = removeDuplicateStr(names)
	if len(ans) != 1 {
		t.Errorf("removeDuplicateStr(['Alice']) = %v; want ['Alice']", ans)
	}

	names = append(names, "Alice")
	ans = removeDuplicateStr(names)
	if len(ans) != 1 {
		t.Errorf("removeDuplicateStr(['Alice']) = %v; want ['Alice']", ans)
	}
}
