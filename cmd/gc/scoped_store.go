package main

import (
	"context"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
)

// scopedBdStoreForCity returns a throwaway BdStore for cityPath whose bd
// subprocess is bound to ctx, so cancellation propagates to the child instead
// of letting it survive past the caller's own budget.
func scopedBdStoreForCity(ctx context.Context, cityPath string) (*beads.BdStore, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	env, err := bdRuntimeEnvWithError(ctx, cityPath)
	if err != nil {
		return nil, err
	}
	runner, err := beadsCommandRunnerForHostedCity(ctx, cityPath, env)
	if err != nil {
		return nil, err
	}
	return beads.NewBdStore(cityPath, runner), nil
}

// scopedBdStoreForRig is scopedBdStoreForCity for a rig-scoped store.
func scopedBdStoreForRig(ctx context.Context, cityPath string, cfg *config.City, rigDir string) (*beads.BdStore, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	env, err := bdRuntimeEnvForRigWithError(ctx, cityPath, cfg, rigDir)
	if err != nil {
		return nil, err
	}
	runner, err := beadsCommandRunnerForHostedCity(ctx, cityPath, env)
	if err != nil {
		return nil, err
	}
	return beads.NewBdStore(rigDir, runner), nil
}

// bdStoreBacking unwraps store through any CachingStore/beadPolicyStore
// layers to find the underlying *beads.BdStore. It returns ok=false for
// stores that aren't bd-CLI-backed (native, file, exec, mem, ...) — those
// have no subprocess to leak, so ga-cdmx6x's mitigation doesn't apply to
// them. Bounded to a handful of iterations: real store stacks are only a few
// layers deep (normally beadPolicyStore wrapping CachingStore wrapping the
// raw store); the bound just guards against an unexpected wrap cycle.
func bdStoreBacking(store beads.Store) (*beads.BdStore, bool) {
	for range 8 {
		switch v := store.(type) {
		case *beads.BdStore:
			return v, v != nil
		case *beads.CachingStore:
			if v == nil {
				return nil, false
			}
			backing := v.Backing()
			if backing == nil {
				return nil, false
			}
			store = backing
			continue
		}
		if inner, _, ok := unwrapBeadPolicyStore(store); ok {
			store = inner
			continue
		}
		return nil, false
	}
	return nil, false
}

// beadPolicyConfig finds the policy layer, if any, in the same bounded store
// stack understood by bdStoreBacking. A scoped clone must retain this layer:
// policy-aware zero-value List and Ready reads span both logical tiers.
func beadPolicyConfig(store beads.Store) (*config.City, bool) {
	for range 8 {
		if _, policy, ok := unwrapBeadPolicyStore(store); ok {
			return policy.cfg, true
		}
		cached, ok := store.(*beads.CachingStore)
		if !ok || cached == nil || cached.Backing() == nil {
			return nil, false
		}
		store = cached.Backing()
	}
	return nil, false
}

// scopedStoreLike returns a throwaway, ctx-bound clone of existing when
// existing is (or wraps, via CachingStore/beadPolicyStore) a bd-CLI-shell
// backed store: cancellation kills the backend bd subprocess instead of
// abandoning it past ctx's deadline. Returns (nil, nil) when existing is
// not bd-CLI backed — callers should keep reading through existing
// directly in that case (gascity ga-cdmx6x).
func scopedStoreLike(ctx context.Context, cityPath string, cfg *config.City, existing beads.Store) (beads.Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	bs, ok := bdStoreBacking(existing)
	if !ok {
		return nil, nil
	}
	policyCfg, policyWrapped := beadPolicyConfig(existing)
	dir := bs.Dir()
	var scoped beads.Store
	var err error
	if samePath(dir, cityPath) {
		scoped, err = scopedBdStoreForCity(ctx, cityPath)
	} else {
		scoped, err = scopedBdStoreForRig(ctx, cityPath, cfg, dir)
	}
	if err != nil {
		return nil, err
	}
	if policyWrapped {
		scoped = wrapStoreWithBeadPolicies(scoped, policyCfg)
	}
	return scoped, nil
}
