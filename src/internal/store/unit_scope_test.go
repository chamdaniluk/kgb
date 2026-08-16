package store

import (
	"context"
	"testing"
)

func TestUnitInScopeKorwilOnlyIncludesSDAndTK(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	var korwil int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('KORWIL-SCOPE','Korwil Scope','korwil') RETURNING id`).Scan(&korwil); err != nil {
		t.Fatal(err)
	}
	children := []struct {
		code     string
		name     string
		typeName string
		id       *int64
	}{
		{code: "SD-SCOPE", name: "SD Scope", typeName: "sd", id: new(int64)},
		{code: "TK-SCOPE", name: "TK Scope", typeName: "tk", id: new(int64)},
		{code: "SMP-SCOPE", name: "SMP Scope", typeName: "smp", id: new(int64)},
		{code: "SKB-SCOPE", name: "SKB Scope", typeName: "skb", id: new(int64)},
	}
	for _, child := range children {
		if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type,parent_id) VALUES ($1,$2,$3,$4) RETURNING id`, child.code, child.name, child.typeName, korwil).Scan(child.id); err != nil {
			t.Fatal(err)
		}
	}

	checks := []struct {
		name      string
		candidate int64
		want      bool
	}{
		{"sd", *children[0].id, true},
		{"tk", *children[1].id, true},
		{"smp", *children[2].id, false},
		{"skb", *children[3].id, false},
	}
	for _, check := range checks {
		got, err := UnitInScope(ctx, pool, korwil, check.candidate)
		if err != nil {
			t.Fatal(err)
		}
		if got != check.want {
			t.Errorf("Korwil scope %s = %v, want %v", check.name, got, check.want)
		}
	}
}

func TestUnitInScopeSchoolOnlyOwnUnit(t *testing.T) {
	pool := testPool(t)
	resetSchema(t, pool)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	var first, second int64
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('SMP-SCOPE-1','SMP Scope 1','smp') RETURNING id`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO units (code,name,type) VALUES ('SMP-SCOPE-2','SMP Scope 2','smp') RETURNING id`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	got, err := UnitInScope(ctx, pool, first, first)
	if err != nil || !got {
		t.Fatalf("own unit scope = %v, err=%v; want true", got, err)
	}
	got, err = UnitInScope(ctx, pool, first, second)
	if err != nil || got {
		t.Fatalf("other unit scope = %v, err=%v; want false", got, err)
	}
}

var _ = context.Background

// END
