import { defineConfig } from "@playwright/test";
export default defineConfig({ testDir: "/Volumes/data/live_commerce_architecture_v1/.worktrees/tz-audit/tests/admin", testMatch: ["ads.spec.ts", "design.spec.ts"], reporter: [["list"]] });
