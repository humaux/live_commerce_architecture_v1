# CVS-SNAP #262 continuation: ownership and field-error locator

Authority: current continuation card; cxf-262-fix2.md approvals retained; architecture sections0,1,2,26,27 and contracts/taiwan-cvs-logistics-v1.md F21/16.1.

Current-head CI evidence: Foundation40762185265/job123076746838 reports generated ownership drift. Gates40762185213/jobs123081441225 and123081443055 fail solely on CVS browser children. Artifacts12852401100/12853786208 show a422 store_not_found followed by strict-mode failure at cvs-buyer.mjs:296: the global alert locator also selects Next.js's route announcer. This is E3 historical failing evidence bound to head17bf02dbc4e3c96e449bb857f038e05d8959c31f and merge5dbb2290fe7fb9dee8b00c3d76d2cb5e8b940589, not acceptance of this repair.

Repair: bind the existing localized error expectation to the store-code label, preserving strict locator semantics, all three locales and real clicks. Merge current integration c12fac9a2acafc04a7be5222c10ff2573975dde6 with no conflicts, then regenerate53-mode ownership from the merged inputs. No product source, migration number, R2 pin, gate assertions, exemption or threshold changed in this continuation. No separate tests-only push: this is repair of the existing RED card.

New path outside original write paths: output/ci-select-backend-browser/browser-ownership.json (explicitly authorized generated artifact). Earlier approved exceptions remain listed in pr-body.md. All other integration-merge paths originate in already-merged #259/#272 and are not lane-authored changes.

Static checks: node --check tests/storefront/cvs-buyer.mjs and git diff --check (changed source/generated artifact): exit0. Ownership regeneration: exit0,53 modes. Local tests, browsers, PG and servers NOT_RUN as required. Current-head CI, independent review, SANDBOX/LIVE/production remain PENDING/NOT_RUN. Author PASS E0; no completion claim.

Rollback: premerge head17bf02dbc4e3c96e449bb857f038e05d8959c31f; original tracked dirty diff preserved in fix3-premerge.patch. Preserve inherited untracked evidence. Revert reviewed commits rather than reset unrelated work. No merge/auto-merge to integration or production action.

No own local processes remain after downloads are closed; redundant partial ZIPs removed after selected original failure logs were independently fetched by HTTP Range. Integrator relays a fresh session when registered CI workflows finish. Final run/job IDs go in a PR comment, with no evidence-only push after GREEN.
