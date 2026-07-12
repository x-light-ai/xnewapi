# Semi Windows Path Fix Upstream Review

- Target checked: `upstream/main` at `4e570389dd433a717373ce9c9b822b59f5ed3d5d`.
- Result: no upstream patch is applicable.
- Reason: the current upstream frontend uses Rsbuild in `web/classic/rsbuild.config.ts`, not Vite or `@douyinfe/vite-plugin-semi`.
- Upstream already resolves the Semi UI package path explicitly with `path.resolve`, so the legacy Vite Windows path failure is structurally removed.
- Fork action: keep `web/vite-plugin-semi-path-safe.js` only while this fork remains on the legacy Vite frontend; delete it when adopting the upstream Rsbuild frontend.
