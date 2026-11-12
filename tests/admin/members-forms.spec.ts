// Purpose: native M7 create/adjust acceptance, isolated from the original immutable members read corpus.
// Depends on: TestBrowserW6Customers members-forms production Next/Go/PG subtest, signed IdP, keyed fixture controls.
// Used by: --browser-customers-billing; controls prepare faults/read SQL only, never replace UI mutation actions.
import { test, expect, type Browser, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import path from "node:path";
import {
  memberFormCopy,
  membersCopy,
} from "../../apps/admin/components/members/copy";
import { shellCopy } from "../../apps/admin/src/shell-copy";
const required = (key: string) => {
  const value = process.env[key];
  if (!value) throw Error(`${key} required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"),
  evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_W6UI_FORMS_STORE"),
  alt = required("LC_W6UI_FORMS_ALT"),
  foreign = required("LC_W6UI_FOREIGN_STORE");
const foreignMember = required("LC_W6UI_FOREIGN_MEMBER");
const control = required("LC_W6UI_CONTROL_URL"),
  gateKey = required("LC_W6UI_CONTROL_KEY");
const sourceRoles = {
  CreateDesktop: {
    frame: "1681:52319",
    modal: "1681:52392",
    propertySHA256:
      "28c9688dece774a16f8b987203d4d46fb1f4b5f18590df41c918251030ea0206",
    pngSHA256:
      "ce507e77083bfb66af2041e8cc1de105c006211b6b06399cc9736e31d13d1dda",
    fields: {
      name: 45,
      contact: 45,
      notes: 88,
    },
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 476,
    radius: 12,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  CreateMobile: {
    frame: "1681:52423",
    modal: "1681:52456",
    propertySHA256:
      "dac1c4875658cfd6baab6f30e20873edb9ccd9f28115434eb83b1976852420af",
    pngSHA256:
      "1888b574a11df73e9f7b0d0692f46ef759356adba3a077c5c90f4c51241cf57c",
    fields: {
      name: 45,
      contact: 45,
      notes: 45,
    },
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 425,
    radius: 16,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  CreateEmptyNameMobile: {
    frame: "1681:52478",
    modal: "1681:52511",
    propertySHA256:
      "04a6a5af376ecc96ba992558e072b7451c7bb298b89a94e24e6527da95ae4bde",
    pngSHA256:
      "6353483ae0748332b9a13fdc53e2b5eb00377853f6a2f2906513abeace02cd2a",
    fields: {
      name: 46,
      contact: 45,
      notes: 45,
    },
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 452,
    radius: 16,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  AdjustDesktop: {
    frame: "1681:52534",
    modal: "1681:52613",
    propertySHA256:
      "e1e6ad65d28e3209c5264431dbb6ea8dbb4805f7f9b3c539c6b3e1c8be258b5e",
    pngSHA256:
      "91770478806278b79420ed3951da7a35c2bf11ce9b6cc3496b1a2d761f8b30f0",
    fields: {
      amount: 45,
      note: 88,
    },
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 523,
    radius: 12,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  AdjustMobile: {
    frame: "1681:52652",
    modal: "1681:52687",
    propertySHA256:
      "54f42077c2dd2bdd981ce21b4fb2965a605dc4c48f8de0160930e8f5181ce453",
    pngSHA256:
      "3608202dd6cdabd54b3dbb020ed369b34dc2a03397d02710b030ae73d1db4be7",
    fields: {
      amount: 45,
      note: 88,
    },
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 515,
    radius: 16,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  AdjustEmptyNoteMobile: {
    frame: "1681:52716",
    modal: "1681:52751",
    propertySHA256:
      "bc6dd1a5682c46114558f239c495284c9f636071e7b7a1d545208ebfe7beb566",
    pngSHA256:
      "c65cc5a5477447123b4d652a83ad14522ccdc640361516f302bc90cb044fa400",
    fields: {
      amount: 45,
      note: 46,
    },
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 436,
    radius: 16,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  AdjustInsufficientMobile: {
    frame: "1681:52779",
    modal: "1681:52814",
    propertySHA256:
      "96fa5270b561c67ae90486dc734a0fdce2d4910945d582f7e040acdaf69c7f27",
    pngSHA256:
      "7bbd4657f297f6db68a5cc12e71ae8c90089d312646394969ef90f44f3bdbd6f",
    fields: {
      amount: 46,
      note: 45,
    },
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 436,
    radius: 16,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  AdjustSubmittingMobile: {
    frame: "1681:52842",
    modal: "1681:52877",
    propertySHA256:
      "25dc36f24fd92290a99cf52f9f936829f13b9d02de372a172ba33c565366a389",
    pngSHA256:
      "4739e39d2509533ccb171db585143f8111a30574b20826b91e2517f8a724cbfc",
    fields: {},
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 325,
    radius: 16,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  CreateSubmittingDesktop: {
    frame: "1681:52894",
    modal: "1681:52967",
    propertySHA256:
      "33b0d886c004df54f9ccfcf8585b882663f8c78e5a0feb478ea377bd8337bd03",
    pngSHA256:
      "2881cff8dd50cb5cf64b4deb29638a53138b954982324fcf21110f6385877863",
    fields: {
      name: 45,
      contact: 45,
      notes: 88,
    },
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 512,
    radius: 12,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  CreateSubmittingMobile: {
    frame: "1681:52999",
    modal: "1681:53032",
    propertySHA256:
      "3ebf6c4d1240f619843ad747d492fcf66c92a22ade7727d28a1216d0088b83d3",
    pngSHA256:
      "1ac821f7bf257e350e6b44959b471ab60ffa4f091baec15eea3bc5aad68c9985",
    fields: {
      name: 45,
      contact: 45,
      notes: 45,
    },
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 461,
    radius: 16,
    authority: "66ffde1e",
    propertyFileSHA256:
      "3ea7073f964713462dc82f20f2bd9242a882712c3942e3d2fb4fa108c217b84c",
  },
  CreateEmptyNameDesktop: {
    frame: "1713:52478",
    modal: "1713:52551",
    propertySHA256:
      "17f8c66d470890dd3be5904b7a4cd976d91d8411541c7bf935047e0e3f951d87",
    pngSHA256:
      "0e027e7094de4091d66cfa9b2b80ec572f39a34b905f778f04cb847efd29a0e8",
    fields: {
      name: 46,
      contact: 45,
      notes: 88,
    },
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 503,
    radius: 12,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  AdjustEmptyNoteDesktop: {
    frame: "1713:52583",
    modal: "1713:52662",
    propertySHA256:
      "354c0d15195b9df8c3f0ac2a606f85165c531d4ff3e930ab496c14385666cd49",
    pngSHA256:
      "ac85f716e1777a1978270c8e3735564bb0dc9cb3e8e6601e9c963573ba59b3d7",
    fields: {
      amount: 45,
      note: 88,
    },
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 486,
    radius: 12,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  AdjustInsufficientDesktop: {
    frame: "1713:52700",
    modal: "1713:52779",
    propertySHA256:
      "46af33a0d4b1b6ee7ace31fe4a728d4e4860116525db4bed6761a307bb3c4864",
    pngSHA256:
      "05b2550afbff0426d391ceadf12db8e549f69ce32d3623d42139fe49ab72ef2e",
    fields: {
      amount: 46,
      note: 88,
    },
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 487,
    radius: 12,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  AdjustSubmittingDesktop: {
    frame: "1718:52529",
    modal: "1718:52608",
    propertySHA256:
      "6a2cb3c850bf82631bc0ea01e4ba319f8e92281de6a821a2f0b194e02885edd5",
    pngSHA256:
      "796b7902377582e0caefb9505e0bd1575f75de9279e538f374ad5f19b64e32b0",
    fields: {},
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 333,
    radius: 12,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  RefusedDesktop: {
    frame: "1558:84734",
    modal: "1558:84848",
    propertySHA256:
      "e383eaac2bd3bb024b9f32d2e8781b6bf1dd29357674e191632a8fc5078830b2",
    pngSHA256:
      "2b3c4a0126a4a339653ceb6fd0875be8fe6100602fd6b4587842994fe5a87dbb",
    fields: {},
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 219,
    radius: 12,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  RefusedMobile: {
    frame: "1558:84857",
    modal: "1558:84914",
    propertySHA256:
      "99c4bf0e0aff108bb40d5a96e3da466dcfa1cd6fb8230bb715805dbe5daa9507",
    pngSHA256:
      "995179fa3989c560c9e3883339ddad6bebcf95fec7227ea4222aa1d6516fccea",
    fields: {},
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 232,
    radius: 16,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  CreateUnknownDesktop: {
    frame: "1666:52314",
    modal: "1666:52407",
    propertySHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
    pngSHA256:
      "517c37f2f3812ce367ded738d32fa0b05ddcdccdc4feea88f439df49448d2ecd",
    fields: {},
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 257,
    radius: 12,
    authority: "b7740fa0",
    propertyFileSHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
  },
  CreateUnknownMobile: {
    frame: "1666:52427",
    modal: "1666:52480",
    propertySHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
    pngSHA256:
      "125eed10351ea944812a4862b0da78674a14f30f6db84c2a033afd92118271df",
    fields: {},
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 270,
    radius: 16,
    authority: "b7740fa0",
    propertyFileSHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
  },
  CreateRefusedDesktop: {
    frame: "1666:52491",
    modal: "1666:52584",
    propertySHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
    pngSHA256:
      "1e11cabe26d49f0004e5bc4f5a74bf7a0d2659159e28895211d2da04030d08eb",
    fields: {},
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 219,
    radius: 12,
    authority: "b7740fa0",
    propertyFileSHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
  },
  CreateRefusedMobile: {
    frame: "1666:52604",
    modal: "1666:52657",
    propertySHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
    pngSHA256:
      "f81297204910101babfe9b1f7cbd9868e11772700b74d4f4f1299a25730877c1",
    fields: {},
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 211,
    radius: 16,
    authority: "b7740fa0",
    propertyFileSHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
  },
  PositiveConfirmDesktop: {
    frame: "1666:52668",
    modal: "1666:52747",
    propertySHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
    pngSHA256:
      "c4809c895a873879dfcd82456a074c5b566bab344fc56a6f10610f3d92c92200",
    fields: {},
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 333,
    radius: 12,
    authority: "b7740fa0",
    propertyFileSHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
  },
  PositiveConfirmMobile: {
    frame: "1666:52774",
    modal: "1666:52809",
    propertySHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
    pngSHA256:
      "e2178fd16d8c91f20b5ed0bd4278853cc9801812b9899ca51ce53cd779912d2b",
    fields: {},
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 325,
    radius: 16,
    authority: "b7740fa0",
    propertyFileSHA256:
      "515ee71bd52ad613b209c2485ac1b4d193273ddff2d98a21dde6f073fd9ff6b6",
  },
  NegativeConfirmDesktop: {
    frame: "1558:84155",
    modal: "1558:84269",
    propertySHA256:
      "8f905258c2db61ec2d3a1b35fdef8de4a2148ee0e1a460fa8374da762a3eb857",
    pngSHA256:
      "1c54bed0e9f071e788d45603465cc18cb4895cd996bb0c558a0d652b6bef4760",
    fields: {},
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 333,
    radius: 12,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  NegativeConfirmMobile: {
    frame: "1558:84281",
    modal: "1558:84338",
    propertySHA256:
      "a4ed45c1ce15ca35673c265fc9f988f91d4805664050b520c4afe8d9770f944b",
    pngSHA256:
      "15b3c0a2af71863c969aa1958920d48d61e73136e520de37f069c5752bb64791",
    fields: {},
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 325,
    radius: 16,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  UnknownDesktop: {
    frame: "1558:84545",
    modal: "1558:84659",
    propertySHA256:
      "ce5939593dc3ba995ab5834cb25a296f0258af51cd42169afc960854b9ad2d69",
    pngSHA256:
      "bfa3469807835a9672dc1258b1503e258c28c8f0c78426c88f244b7be1c5d465",
    fields: {},
    padding: [28, 28, 28, 28],
    width: 520,
    referenceHeight: 278,
    radius: 12,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
  UnknownMobile: {
    frame: "1558:84668",
    modal: "1558:84725",
    propertySHA256:
      "40cc2c76291aaf27c8d87b5e0db8394d2653bd30241417dcced1fa2f05258171",
    pngSHA256:
      "d74f6c190d38152be04d06af48c30955dcbda40c894184ee0da0d29a475058d2",
    fields: {},
    padding: [20, 16, 28, 16],
    width: 390,
    referenceHeight: 270,
    radius: 16,
    authority: "41e8d438",
    propertyFileSHA256:
      "c577e75f1e3a627046e1f7af7af330fe5bfcfc8a1a08c50632af1175e9ace9ae",
  },
} as const;
const originalCases = [
  "REAL_PG forms en/1440",
  "REAL_PG forms en/375",
  "REAL_PG forms zh-TW/1440",
  "REAL_PG forms zh-TW/375",
  "REAL_PG forms zh-CN/1440",
  "REAL_PG forms zh-CN/375",
  "REAL_PG forms reader permission and opaque foreign refusal",
  "REAL_PG forms create pending double click then committed UNKNOWN never resends",
  "REAL_PG forms adjust committed UNKNOWN reads SQL effect exactly once",
  "REAL_PG forms held commit suppresses stale store and logout completion",
  "REAL_PG forms genuine server race refusal and correction controls",
  "REAL_PG forms same-tab store history boundary",
  "REAL_PG forms same-tab member history boundary",
  "REAL_PG forms reauthenticated session epoch",
  "REAL_PG forms erased before open",
  "REAL_PG forms erased after review",
  "REAL_PG forms unavailable member read state",
  "REAL_PG forms create pending double click then committed UNKNOWN never resends zh-TW",
  "REAL_PG forms create pending double click then committed UNKNOWN never resends zh-CN",
  "REAL_PG forms adjust committed UNKNOWN reads SQL effect exactly once zh-TW",
  "REAL_PG forms adjust committed UNKNOWN reads SQL effect exactly once zh-CN",
  "REAL_PG forms genuine server race refusal and correction controls zh-TW",
  "REAL_PG forms genuine server race refusal and correction controls zh-CN",
  "REAL_PG forms genuine create permission refusal en",
  "REAL_PG forms genuine create permission refusal zh-TW",
  "REAL_PG forms genuine create permission refusal zh-CN",
] as const;
type Locale = "en" | "zh-TW" | "zh-CN";
type Row = {
  page: string;
  control: string;
  action: string;
  expected: string;
  actual: string;
  result: "pass" | "fail";
};
type SQLRow = {
  actor: string;
  store: string;
  member: number;
  case: string;
  phase: string;
  seq: number;
  counts: Record<string, number>;
  ui_state: string;
  session_changed: boolean;
  key_body_unchanged: boolean;
};
const sqlRows: SQLRow[] = [],
  geometryRows: {
    case: string;
    role: string;
    passed: boolean;
    source: unknown;
    measurement: unknown;
    screenshots?: {
      viewport: { file: string; sha256: string };
      dialog: { file: string; sha256: string };
    };
  }[] = [];
let activeCase = "",
  activePage: Page | null = null,
  sessionChanged = false;
const sourcePin = {
  checkout: required("LC_M7_SOURCE_SHA"),
  specSHA256: required("LC_M7_SPEC_SHA256"),
  goSHA256: required("LC_M7_GO_SHA256"),
};
const ledger: Row[] = [],
  completed: string[] = [];
test.describe.configure({ mode: "serial" });
test.beforeEach(async ({}, info) => {
  const index = (originalCases as readonly string[]).indexOf(info.title);
  expect(index).toBeGreaterThanOrEqual(0);
  activeCase = `C${String(index + 1).padStart(2, "0")}`;
  activePage = null;
  sessionChanged = false;
  await ctl("forms-case", activeCase);
  await state("", "before");
});
test.afterEach(async ({}, info) => {
  await state("", "final");
  if (info.status === "passed") completed.push(info.title);
  await writeFile(
    path.join(evidence, "members-forms-click-ledger.json"),
    JSON.stringify(ledger, null, 2),
  );
});
test.afterAll(async () => {
  await writeFile(
    path.join(evidence, "members-forms-completion.json"),
    JSON.stringify(
      {
        cases: completed,
        classification:
          "REAL_PG native controls; fault/fixture setup separately disclosed",
      },
      null,
      2,
    ),
  );
});
async function step(
  page: Page,
  name: string,
  action: string,
  expected: string,
  run: () => Promise<void>,
) {
  const row: Row = {
    page: new URL(page.url()).pathname,
    control: name,
    action,
    expected,
    actual: "assertion pending",
    result: "fail",
  };
  ledger.push(row);
  try {
    await run();
    row.actual = "user-visible assertion passed";
    row.result = "pass";
  } catch (e) {
    row.actual = "assertion failed; retained native trace";
    throw e;
  }
}
async function ctl(resource: string, mode = "") {
  // Synthetic fixture preparation/fault synchronization only, never a successful UI-write substitute.
  const r = await fetch(`${control}/${resource}`, {
    method: "POST",
    headers: { "X-W6UI-Gate-Key": gateKey, "Content-Type": "application/json" },
    body: JSON.stringify({ mode }),
    signal: AbortSignal.timeout(5000),
  });
  expect(r.status).toBe(204);
}
async function state(
  member = "",
  phase = "snapshot",
): Promise<Record<string, number>> {
  // Independent Go owner-pool SQL readback; contains counts/numbers only, no buyer text or command keys.
  const r = await fetch(`${control}/forms-state`, {
    method: "POST",
    headers: { "X-W6UI-Gate-Key": gateKey, "Content-Type": "application/json" },
    body: JSON.stringify({ mode: `${phase}|${member}` }),
    signal: AbortSignal.timeout(5000),
  });
  expect(r.status).toBe(200);
  const counts = (await r.json()) as Record<string, number>;
  let ui = "closed";
  if (
    activePage &&
    !activePage.isClosed() &&
    (await dialog(activePage).isVisible())
  )
    ui = (await dialog(activePage).getAttribute("data-step")) ?? "unclassified";
  sqlRows.push({
    actor: activeCase === "C07" ? "R" : "P",
    store: "S",
    member: counts.memberAlias,
    case: activeCase,
    phase,
    seq: counts.phaseSeq,
    counts,
    ui_state: ui,
    session_changed: sessionChanged,
    key_body_unchanged: counts.keyBodyUnchanged === 1,
  });
  await writeFile(
    path.join(evidence, "members-forms-sql-ledger.json"),
    JSON.stringify(sqlRows, null, 2),
  );
  return counts;
}
const list = (locale: Locale, scope = store) =>
  `${origin}/${locale}/members?store=${scope}`;
const dialog = (page: Page) => page.getByTestId("member-form-dialog");
async function signed(
  browser: Browser,
  locale: Locale = "en",
  width = 1440,
  actor = "forms-points",
) {
  await ctl("actor", actor);
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    viewport: { width, height: 900 },
    deviceScaleFactor: 1,
  });
  const page = await context.newPage();
  activePage = page;
  await page.goto(`${origin}/en/`);
  await step(
    page,
    "identity sign-in",
    "click",
    "native signed callback",
    async () => {
      await page
        .getByRole("button", { name: "Sign in with identity service" })
        .click();
      await expect(page.getByTestId("shell-store-selector")).toBeVisible();
    },
  );
  await page.goto(list(locale));
  await expect(page.getByTestId("members-list")).toBeVisible();
  return { page, context };
}
async function captureRole(page: Page, locale: Locale, role: string) {
  const width = page.viewportSize()!.width;
  const key =
    `${role}${width === 375 ? "Mobile" : "Desktop"}` as keyof typeof sourceRoles;
  const source = sourceRoles[key];
  expect(source, `source binding for ${key}`).toBeTruthy();
  await expect(dialog(page)).toBeVisible();
  const measurement = await dialog(page).evaluate((el) => {
    const d = el as HTMLDialogElement,
      cs = getComputedStyle(d),
      rect = d.getBoundingClientRect();
    const box = (e: HTMLElement) => {
      const r = e.getBoundingClientRect();
      return {
        x: r.x,
        y: r.y,
        width: r.width,
        height: r.height,
        clientWidth: e.clientWidth,
        scrollWidth: e.scrollWidth,
      };
    };
    const tokenNames = [
      "--ds-control-member-dialog-width",
      "--ds-control-member-dialog-pad",
      "--ds-radius-member-dialog",
      "--ds-radius-member-sheet",
      "--ds-control-member-field-gap",
      "--ds-text-member-value",
      "--ds-text-member-heading",
      "--ds-control-member-action-leading",
      "--ds-members-control-bg",
      "--ds-control-member-input-pad-block",
      "--ds-radius-member-control",
      "--ds-control-member-note-height",
      "--ds-members-action-bg",
      "--ds-members-action-border",
      "--ds-control-member-action-width",
      "--ds-members-scrim",
      "--ds-border-input",
      "--ds-target",
    ];
    // Detached canvas parses colors without changing the UI. Keep RGB exact and
    // compare alpha as an 8-bit value: 45% and compiled hex alpha 0x73 agree.
    // Only parse the browser's own legacy rgb(a) serialization, never CSS input.
    const color = document.createElement("canvas").getContext("2d")!;
    const rgba8 = (value: string) => {
      if (!CSS.supports("color", value)) throw new Error("Invalid scrim color");
      color.fillStyle = value;
      const serialized = color.fillStyle;
      const rgba = /^rgba?\((\d+), (\d+), (\d+)(?:, ([\d.]+))?\)$/.exec(serialized);
      if (!rgba) throw new Error(`Unexpected scrim serialization: ${serialized}`);
      return [Number(rgba[1]), Number(rgba[2]), Number(rgba[3]),
        Math.round(Number(rgba[4] ?? 1) * 255)];
    };
    const scrim = {
      actual: rgba8(cs.getPropertyValue("--ds-members-scrim").trim()),
      expected: rgba8("rgb(20 41 66 / 45%)"),
      // Spec-local negative controls exercise the same browser parser/comparison.
      wrongRGB: rgba8("rgb(21 41 66 / 45%)"),
      wrongAlpha: rgba8("rgb(20 41 66 / 46%)"),
    };
    return {
      scrim,
      rect: box(d),
      viewport: innerWidth,
      dpr: devicePixelRatio,
      zoom: visualViewport?.scale,
      padding: [
        cs.paddingTop,
        cs.paddingRight,
        cs.paddingBottom,
        cs.paddingLeft,
      ].map(Number.parseFloat),
      radius: Number.parseFloat(cs.borderTopLeftRadius),
      tokens: Object.fromEntries(
        tokenNames.map((n) => [n, cs.getPropertyValue(n).trim()]),
      ),
      focus:
        document.activeElement?.getAttribute("name") ??
        document.activeElement?.getAttribute("data-testid") ??
        document.activeElement?.tagName,
      fields: Array.from(
        d.querySelectorAll<HTMLInputElement | HTMLTextAreaElement>(
          "input,textarea",
        ),
      ).map((e) => ({
        name: e.name,
        rect: box(e),
        container: box(e.parentElement!),
        disabled: e.matches(":disabled"),
        invalid: e.getAttribute("aria-invalid"),
        label: !!document.querySelector(`label[for="${e.id}"]`),
        errorAssociation:
          e.getAttribute("aria-invalid") !== "true" ||
          !!document.getElementById(e.getAttribute("aria-describedby") ?? ""),
      })),
      actions: Array.from(d.querySelectorAll<HTMLButtonElement>("button")).map(
        (e) => ({
          alias: e.dataset.testid,
          rect: box(e),
          disabled: e.disabled || e.getAttribute("aria-disabled") === "true",
        }),
      ),
      x: rect.x,
      y: rect.y,
    };
  });
  const ordinal = geometryRows.filter((r) => r.case === activeCase).length;
  const row = {
    case: activeCase,
    role,
    passed: false,
    source: { ...source, ...sourcePin, locale, viewport: width },
    measurement,
  };
  geometryRows.push(row);
  const prefix = `members-forms-${activeCase}-${role}-${width}-${ordinal}`;
  const viewportPNG = await page.screenshot({
    path: path.join(evidence, `${prefix}-viewport.png`),
  });
  const dialogPNG = await dialog(page).screenshot({
    path: path.join(evidence, `${prefix}-dialog.png`),
  });
  Object.assign(row, {
    screenshots: {
      viewport: {
        file: `${prefix}-viewport.png`,
        sha256: createHash("sha256").update(viewportPNG).digest("hex"),
      },
      dialog: {
        file: `${prefix}-dialog.png`,
        sha256: createHash("sha256").update(dialogPNG).digest("hex"),
      },
    },
  });
  await writeFile(
    path.join(evidence, "members-forms-geometry.json"),
    JSON.stringify(geometryRows, null, 2),
  );
  expect(measurement.dpr).toBe(1);
  expect(measurement.zoom).toBe(1);
  expect(
    Math.abs(measurement.rect.width - (width === 375 ? 375 : source.width)),
  ).toBeLessThanOrEqual(0.5);
  expect(measurement.rect.scrollWidth).toBeLessThanOrEqual(
    measurement.rect.clientWidth,
  );
  source.padding.forEach((v, i) =>
    expect(Math.abs(measurement.padding[i] - v)).toBeLessThanOrEqual(0.5),
  );
  expect(Math.abs(measurement.radius - source.radius)).toBeLessThanOrEqual(0.5);
  const expectedTokens = {
    "--ds-control-member-dialog-width": "520px",
    "--ds-control-member-dialog-pad": "28px",
    "--ds-radius-member-dialog": "12px",
    "--ds-radius-member-sheet": "16px",
    "--ds-control-member-field-gap": "6px",
    "--ds-text-member-value": "15px",
    "--ds-text-member-heading": "20px",
    "--ds-control-member-action-leading": "20px",
    "--ds-members-control-bg": "#f1f5f9",
    "--ds-control-member-input-pad-block": "10px",
    "--ds-radius-member-control": "5px",
    "--ds-control-member-note-height": "88px",
    "--ds-members-action-bg": "#247965",
    "--ds-members-action-border": "#dde5ee",
    "--ds-control-member-action-width": "160px",
    "--ds-members-scrim": "rgb(20 41 66 / 45%)",
    "--ds-border-input": "#8c8c8c",
    "--ds-target": "44px",
  };
  // Raw custom-property text stays in geometry evidence; all other tokens keep
  // their existing exact comparison. Scrim equivalence must reject RGB/alpha drift.
  const { "--ds-members-scrim": rawScrim, ...otherTokens } = measurement.tokens;
  const { "--ds-members-scrim": sourceScrim, ...otherExpectedTokens } = expectedTokens;
  expect(rawScrim).toBeTruthy();
  expect(sourceScrim).toBe("rgb(20 41 66 / 45%)");
  expect(otherTokens).toEqual(otherExpectedTokens);
  expect(measurement.scrim.expected).toEqual([20, 41, 66, 115]);
  expect(measurement.scrim.wrongRGB).not.toEqual(measurement.scrim.expected);
  expect(measurement.scrim.wrongAlpha).not.toEqual(measurement.scrim.expected);
  expect(measurement.scrim.actual).toEqual(measurement.scrim.expected);
  expect(measurement.fields.map((f) => f.name).sort()).toEqual(
    Object.keys(source.fields).sort(),
  );
  for (const f of measurement.fields) {
    const height = (source.fields as Record<string, number>)[f.name];
    expect(Math.abs(f.rect.height - height)).toBeLessThanOrEqual(0.5);
    expect(f.rect.height).toBeGreaterThanOrEqual(44);
    expect(f.container.scrollWidth).toBeLessThanOrEqual(
      f.container.clientWidth,
    );
    expect(f.label).toBe(true);
    expect(f.errorAssociation).toBe(true);
    if (role.endsWith("Submitting")) expect(f.disabled).toBe(true);
  }
  for (const a of measurement.actions) {
    expect(a.rect.height).toBeGreaterThanOrEqual(44);
    expect(a.rect.width).toBeGreaterThanOrEqual(44);
    expect(a.rect.x).toBeGreaterThanOrEqual(measurement.x);
    expect(a.rect.x + a.rect.width).toBeLessThanOrEqual(
      measurement.x + measurement.rect.width,
    );
  }
  row.passed = true;
  await writeFile(
    path.join(evidence, "members-forms-geometry.json"),
    JSON.stringify(geometryRows, null, 2),
  );
  await state("", "geometry");
}
async function captureResponsive(page: Page, locale: Locale, role: string) {
  const before = page.viewportSize()!;
  await captureRole(page, locale, role);
  await page.setViewportSize({ width: 375, height: 900 });
  await captureRole(page, locale, role);
  await page.setViewportSize(before);
}
const matrixCase = () => Number(activeCase.slice(1)) <= 6;

async function fill(page: Page, name: string, value: string) {
  const field = page.locator(`[name="${name}"]`);
  await step(page, name, "fill", "entered synthetic value", async () => {
    await field.fill(value);
    await expect(field).toHaveValue(value);
  });
}
async function create(
  page: Page,
  locale: Locale,
  marker: string,
  exercise = false,
) {
  const c = memberFormCopy[locale],
    before = await state();
  await step(
    page,
    "create opener",
    "click",
    "modal and initial field focus",
    async () => {
      await page.getByTestId("member-create-open").click();
      await expect(dialog(page)).toBeVisible();
      await expect(page.locator('[name="name"]')).toBeFocused();
    },
  );
  if (matrixCase()) await captureRole(page, locale, "Create");
  if (exercise) {
    await step(
      page,
      "create validation",
      "click",
      "first invalid field focused",
      async () => {
        await page.getByTestId("member-form-submit").click();
        await expect(page.locator('[name="name"]')).toBeFocused();
        await expect(dialog(page).getByRole("alert")).toContainText(
          c.invalidName,
        );
      },
    );
    if (matrixCase()) await captureRole(page, locale, "CreateEmptyName");
    await step(
      page,
      "draft cancel",
      "keyboard Escape",
      "session retained and opener focused",
      async () => {
        await page.keyboard.press("Escape");
        await expect(dialog(page)).not.toBeVisible();
        await expect(page.getByTestId("member-create-open")).toBeFocused();
      },
    );
    await page.getByTestId("member-create-open").click();
    await step(
      page,
      "cancel button",
      "click",
      "draft canceled without signing out",
      async () => {
        await page.getByTestId("member-form-cancel").click();
        await expect(dialog(page)).not.toBeVisible();
        await expect(page.getByTestId("shell-store-selector")).toBeVisible();
      },
    );
    await page.getByTestId("member-create-open").click();
  }
  await fill(page, "name", `M7 Synthetic ${marker}`);
  await fill(page, "contact", marker);
  await fill(
    page,
    "notes",
    "Synthetic long English note for the member form layout.",
  );
  if (matrixCase()) {
    await page.locator('[name="name"]').fill("M7_" + "L".repeat(57));
    await captureRole(page, locale, "Create");
    await page.locator('[name="name"]').fill(`M7 Synthetic ${marker}`);
    await ctl("fault", "forms-hold");
  }
  await step(
    page,
    "create submit",
    "click",
    "one acknowledged create and modal closed",
    async () => {
      const settled = page.waitForResponse(
        (r) =>
          r.request().method() === "POST" &&
          new URL(r.url()).pathname === `/api/stores/${store}/members`,
      );
      await page.getByTestId("member-form-submit").click();
      if (matrixCase()) {
        await expect.poll(async () => (await state()).held).toBe(1);
        await captureRole(page, locale, "CreateSubmitting");
        await ctl("forms-release");
      }
      expect((await settled).status()).toBe(201);
      await expect(dialog(page)).not.toBeVisible();
    },
  );
  await step(
    page,
    "contact search",
    "fill/click",
    "created member found through actual search",
    async () => {
      await page.getByTestId("members-search-input").fill(marker);
      await page.getByTestId("members-search-submit").click();
      await expect(
        page
          .getByTestId("members-list")
          .getByRole("link", { name: `M7 Synthetic ${marker}`, exact: true }),
      ).toBeVisible();
    },
  );
  const link = page
    .getByTestId("members-list")
    .getByRole("link", { name: `M7 Synthetic ${marker}`, exact: true });
  const href = await link.getAttribute("href"),
    id = href!.split("/members/")[1].split("?")[0];
  await step(
    page,
    "created member",
    "click/reload",
    "persistent detail after browser refresh",
    async () => {
      await link.click();
      await expect(page.getByTestId("member-detail")).toContainText(marker);
      await page.reload();
      await expect(page.getByTestId("member-detail")).toContainText(marker);
    },
  );
  const after = await state(id);
  expect(after.members).toBe(before.members + 1);
  expect(after.commands).toBe(before.commands + 1);
  expect(after.balance).toBe(0);
  if (matrixCase()) await state(id, "create_ack");
  return id;
}
async function review(
  page: Page,
  locale: Locale,
  amount: string,
  negative = false,
) {
  await step(
    page,
    "adjust opener",
    "click",
    "member adjustment fields visible",
    async () => {
      await page.getByTestId("member-adjust-open").click();
      await expect(dialog(page)).toBeVisible();
    },
  );
  await step(
    page,
    "direction",
    "selectOption",
    "correct addition/deduction selected",
    async () => {
      await page.locator('[name="sign"]').selectOption(negative ? "-1" : "1");
      await expect(page.locator('[name="sign"]')).toHaveValue(
        negative ? "-1" : "1",
      );
    },
  );
  await fill(page, "amount", amount);
  if (matrixCase()) {
    await captureRole(page, locale, "Adjust");
    const before = await state();
    for (const [value, phase] of [
      ["", "note_blank"],
      ["   ", "note_space"],
    ] as const) {
      await fill(page, "note", value);
      await page.getByTestId("member-form-submit").click();
      await expect(page.locator('[name="note"]')).toBeFocused();
      await expect(page.locator('[name="note"]')).toHaveAttribute(
        "aria-invalid",
        "true",
      );
      await expect(dialog(page).getByRole("alert")).toContainText(
        memberFormCopy[locale].invalidNote,
      );
      const after = await state("", phase);
      expect(after.writeRequests).toBe(before.writeRequests);
      expect(after.commands).toBe(before.commands);
      expect(after.entries).toBe(before.entries);
      await captureRole(page, locale, "AdjustEmptyNote");
    }
  }
  await fill(page, "note", "Synthetic adjustment reason");
  await step(
    page,
    "adjust review",
    "click",
    "explicit before/after confirmation",
    async () => {
      await page.getByTestId("member-form-submit").click();
      await expect(dialog(page)).toHaveAttribute("data-step", "confirm");
      await expect(dialog(page)).toContainText(
        memberFormCopy[locale].irreversible,
      );
    },
  );
}
async function confirm(page: Page, balance: number) {
  const locale = new URL(page.url()).pathname.split("/")[1] as Locale;
  if (matrixCase()) {
    await captureRole(
      page,
      locale,
      balance === 50 ? "PositiveConfirm" : "NegativeConfirm",
    );
    await ctl("fault", "forms-hold");
  }
  await step(
    page,
    "adjust confirm",
    "click",
    "authoritative fresh visible balance and ledger",
    async () => {
      await page.getByTestId("member-form-confirm").click();
      if (matrixCase()) {
        await expect.poll(async () => (await state()).held).toBe(1);
        await captureRole(page, locale, "AdjustSubmitting");
        await ctl("forms-release");
      }
      await expect(dialog(page)).not.toBeVisible();
      // Calibration seam: removing onSaved/read.reload must fail this assertion BEFORE hard reload.
      await expect(
        page.getByTestId("member-detail").locator("strong"),
      ).toHaveText(String(balance));
      await expect(page.getByTestId("member-ledger")).toContainText(
        "Synthetic adjustment reason",
      );
      await page.reload();
      await expect(
        page.getByTestId("member-detail").locator("strong"),
      ).toHaveText(String(balance));
    },
  );
  if (matrixCase()) await state("", balance === 50 ? "adjust50" : "adjust30");
}
for (const locale of ["en", "zh-TW", "zh-CN"] as const)
  for (const width of [1440, 375])
    test(`REAL_PG forms ${locale}/${width}`, async ({ browser }) => {
      const { page, context } = await signed(browser, locale, width);
      const marker = `M7_SYNTHETIC_${locale}_${width}`;
      const urls: string[] = [],
        messages: string[] = [];
      page.on("request", (r) => urls.push(r.url()));
      page.on("console", (m) => messages.push(m.text()));
      try {
        const id = await create(page, locale, marker, true);
        await review(page, locale, "50");
        await step(
          page,
          "back to edit",
          "click",
          "review returns without mutation",
          async () => {
            await page.getByTestId("member-form-back").click();
            await expect(page.locator('[name="amount"]')).toHaveValue("50");
          },
        );
        await page.getByTestId("member-form-submit").click();
        await confirm(page, 50);
        const first = await state(id);
        expect(first.balance).toBe(50);
        expect(first.sum).toBe(50);
        expect(first.memberEntries).toBe(1);
        await page.getByTestId("member-adjust-open").click();
        await page.locator('[name="sign"]').selectOption("-1");
        await fill(page, "amount", "51");
        await fill(page, "note", "Synthetic adjustment reason");
        await step(
          page,
          "insufficient client amount",
          "click",
          "visible error and no effect",
          async () => {
            await page.getByTestId("member-form-submit").click();
            await expect(dialog(page).getByRole("alert")).toContainText(
              memberFormCopy[locale].insufficient,
            );
            await expect(page.locator('[name="amount"]')).toBeFocused();
          },
        );
        await captureRole(page, locale, "AdjustInsufficient");
        await fill(page, "amount", "20");
        await page.getByTestId("member-form-submit").click();
        await confirm(page, 30);
        const last = await state(id);
        expect(last.balance).toBe(30);
        expect(last.sum).toBe(30);
        expect(last.memberEntries).toBe(2);
        await page.getByTestId("member-adjust-open").click();
        await step(
          page,
          "keyboard containment",
          "keyboard",
          "Tab and ShiftTab stay inside dialog",
          async () => {
            await expect(page.locator('[name="sign"]')).toBeFocused();
            await page.keyboard.press("Shift+Tab");
            await expect(page.getByTestId("member-form-submit")).toBeFocused();
            await page.keyboard.press("Tab");
            await expect(page.locator('[name="sign"]')).toBeFocused();
            await page.keyboard.press("Shift+Tab");
            await expect(page.getByTestId("member-form-submit")).toBeFocused();
            await page.keyboard.press("Escape");
            await expect(page.getByTestId("member-adjust-open")).toBeFocused();
          },
        );
        // Read-only measurement/storage inspection; never DOM mutation or UI-action substitution.
        const hygiene = await page.evaluate(() => ({
          width: document.documentElement.scrollWidth,
          viewport: innerWidth,
          storage: [
            ...Object.values(localStorage),
            ...Object.values(sessionStorage),
          ].join("|"),
        }));
        expect(hygiene.width).toBeLessThanOrEqual(hygiene.viewport);
        expect(hygiene.storage.includes(marker)).toBe(false);
        expect(urls.some((u) => u.includes(marker))).toBe(false);
        expect(messages.some((m) => m.includes(marker))).toBe(false);
        await page.screenshot({
          path: path.join(evidence, `members-forms-${locale}-${width}.png`),
          fullPage: true,
        });
      } finally {
        await context.close();
      }
    });
test("REAL_PG forms reader permission and opaque foreign refusal", async ({
  browser,
}) => {
  const { page, context } = await signed(browser, "en", 375, "forms-reader");
  try {
    await expect(page.getByTestId("member-create-open")).toHaveCount(0);
    const before = await state(),
      csrf = (await context.cookies()).find(
        (c) => c.name === "__Host-commerce_csrf",
      )!.value;
    // Supplemental trust-boundary probes, not UI write acceptance: missing UI action cannot be clicked.
    for (const [scope, status] of [
      [store, 403],
      [foreign, 404],
    ] as const) {
      const r = await page.request.post(
        `${origin}/api/stores/${scope}/members`,
        {
          headers: {
            Origin: origin,
            "X-CSRF-Token": csrf,
            "Idempotency-Key": `m7-forbidden-${scope}`,
          },
          data: {
            display_name: "M7 synthetic forbidden",
            contact_note: null,
            notes: null,
          },
        },
      );
      expect(r.status()).toBe(status);
    }
    const after = await state();
    expect(after.members).toBe(before.members);
    expect(after.commands).toBe(before.commands);
    await page
      .getByTestId("members-list")
      .locator('a[href*="/members/"]')
      .first()
      .click();
    await expect(page.getByTestId("member-detail")).toBeVisible();
    await expect(page.getByTestId("member-adjust-open")).toHaveCount(0);
    await page.reload();
    await expect(page.getByTestId("member-adjust-open")).toHaveCount(0);
    await state("", "reader_detail");
    await page.getByTestId("member-back").click();
    await step(
      page,
      "reader search",
      "click",
      "read UI still works without points actions",
      async () => {
        await page.getByTestId("members-search-submit").click();
        await expect(page.getByTestId("members-list")).toBeVisible();
      },
    );
  } finally {
    await context.close();
  }
});
test("REAL_PG forms create pending double click then committed UNKNOWN never resends", async ({
  browser,
}) => {
  const { page, context } = await signed(browser);
  const before = await state();
  try {
    await page.getByTestId("member-create-open").click();
    await fill(page, "name", "M7 synthetic dropped create");
    await ctl("fault", "forms-drop");
    await step(
      page,
      "create response lost",
      "click",
      "committed create becomes UNKNOWN",
      async () => {
        await page.getByTestId("member-form-submit").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
      },
    );
    await captureResponsive(page, "en", "CreateUnknown");
    const committed = await state("", "unknown");
    expect(committed.members).toBe(before.members + 1);
    expect(committed.commands).toBe(before.commands + 1);
    await step(
      page,
      "UNKNOWN read only",
      "click/keyboard",
      "refresh and Escape cannot resend or resolve intent",
      async () => {
        await page.getByTestId("member-form-refresh").click();
        await page.keyboard.press("Escape");
        await expect(dialog(page)).toBeVisible();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
        await expect(page.getByTestId("member-form-submit")).toHaveCount(0);
      },
    );
    await state("", "refreshed");
    await state("", "escape_blocked");
    await expect(page.getByTestId("member-form-close")).toHaveCount(0);
    await expect(page.getByTestId("member-form-cancel")).toHaveCount(0);
    const after = await state();
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
  } finally {
    await context.close();
  }
});
test("REAL_PG forms adjust committed UNKNOWN reads SQL effect exactly once", async ({
  browser,
}) => {
  const { page, context } = await signed(browser);
  try {
    const id = await create(page, "en", "M7_UNKNOWN_ADJUST");
    await review(page, "en", "5");
    const before = await state(id);
    await ctl("fault", "forms-drop");
    await page.getByTestId("member-form-confirm").click();
    await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
    await captureResponsive(page, "en", "Unknown");
    const committed = await state(id, "unknown");
    expect(committed.balance).toBe(5);
    expect(committed.sum).toBe(5);
    expect(committed.memberEntries).toBe(1);
    expect(committed.commands).toBe(before.commands + 1);
    await step(
      page,
      "adjust UNKNOWN refresh",
      "click",
      "read refresh keeps unresolved state and sends no POST",
      async () => {
        await page.getByTestId("member-form-refresh").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
      },
    );
    await state(id, "refreshed");
    await page.keyboard.press("Escape");
    await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
    await state(id, "escape_blocked");
    await expect(page.getByTestId("member-form-close")).toHaveCount(0);
    await expect(page.getByTestId("member-form-cancel")).toHaveCount(0);
    const after = await state(id);
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
  } finally {
    await context.close();
  }
});
test("REAL_PG forms held commit suppresses stale store and logout completion", async ({
  browser,
}) => {
  const { page, context } = await signed(browser);
  const before = await state();
  try {
    await page.getByTestId("member-create-open").click();
    await fill(page, "name", "M7_HELD_PRIVATE");
    await ctl("fault", "forms-hold");
    await step(
      page,
      "pending rapid submit",
      "dblclick/Enter/Escape",
      "one dispatch and inputs/cancel locked",
      async () => {
        await page.getByTestId("member-form-submit").dblclick();
        await expect(dialog(page)).toHaveAttribute("data-step", "pending");
        await page.getByTestId("member-form-submit").press("Enter");
        await page.keyboard.press("Escape");
        await expect(dialog(page)).toHaveAttribute("data-step", "pending");
        await expect(page.locator('[name="name"]')).toBeDisabled();
        await expect(page.getByTestId("member-form-cancel")).toHaveAttribute(
          "aria-disabled",
          "true",
        );
      },
    );
    await expect.poll(async () => (await state()).held).toBe(1);
    const committed = await state("", "committed");
    expect(committed.members).toBe(before.members + 1);
    expect(committed.writeRequests).toBe(before.writeRequests + 1);
    // Second native tab shares actual cookies; its controls can change scope/logout while pending dialog is modal.
    const other = await context.newPage();
    await other.goto(list("en"));
    await step(
      other,
      "store selector",
      "selectOption",
      "alternate store hides points entry",
      async () => {
        await other.getByTestId("shell-store-selector").selectOption(alt);
        await other.getByTestId("nav-members").click();
        await expect(other.getByTestId("member-create-open")).toHaveCount(0);
      },
    );
    await step(
      other,
      "native logout",
      "click",
      "authenticated session revoked before held response release",
      async () => {
        await other
          .locator("summary")
          .filter({ hasText: shellCopy.en.account })
          .click();
        await other.getByTestId("workspace-sign-out").click();
        await expect(other).toHaveURL(/\/en\/?$/);
      },
    );
    await ctl("forms-release");
    await state("", "logout");
    await expect(page.getByTestId("member-form")).toHaveCount(0);
    await expect(
      page.getByText("M7_HELD_PRIVATE", { exact: true }),
    ).toHaveCount(0);
    const after = await state();
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
  } finally {
    await ctl("forms-release");
    await context.close();
  }
});
test("REAL_PG forms genuine server race refusal and correction controls", async ({
  browser,
}) => {
  const { page, context } = await signed(browser);
  try {
    const id = await create(page, "en", "M7_RACE");
    await review(page, "en", "50");
    await confirm(page, 50);
    await review(page, "en", "30", true);
    await ctl("forms-race", id);
    const before = await state(id, "race_fixture");
    await step(
      page,
      "server concurrent deduction refusal",
      "click",
      "actual409 visible and no second deduction",
      async () => {
        await page.getByTestId("member-form-confirm").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "refused");
        await expect(dialog(page).getByRole("alert")).toContainText(
          memberFormCopy.en.insufficient,
        );
      },
    );
    await captureResponsive(page, "en", "Refused");
    const after = await state(id, "refused");
    expect(after.commands).toBe(before.commands);
    expect(after.memberEntries).toBe(before.memberEntries);
    expect(after.balance).toBe(0);
    await step(
      page,
      "refusal back",
      "click",
      "draft can be corrected",
      async () => {
        await page.getByTestId("member-form-back").click();
        await expect(page.locator('[name="amount"]')).toHaveValue("30");
      },
    );
    await page.getByTestId("member-form-submit").click();
    await page.getByTestId("member-form-confirm").click();
    await expect(dialog(page)).toHaveAttribute("data-step", "refused");
    await step(
      page,
      "refusal close",
      "click",
      "modal closes and opener focused",
      async () => {
        await page.getByTestId("member-form-close").click();
        await expect(dialog(page)).not.toBeVisible();
        await expect(page.getByTestId("member-adjust-open")).toBeFocused();
      },
    );
  } finally {
    await context.close();
  }
});

async function heldCreate(page: Page, label: string) {
  await page.getByTestId("member-create-open").click();
  await fill(page, "name", label);
  await ctl("fault", "forms-hold");
  await page.getByTestId("member-form-submit").click();
  await expect(dialog(page)).toHaveAttribute("data-step", "pending");
  await expect.poll(async () => (await state()).held).toBe(1);
  return state("", "committed");
}
async function nativeBack(page: Page, expectedURL: string) {
  // Traverse one real history entry; URL/state, not a possibly null response, proves arrival.
  expect(page.url()).not.toBe(expectedURL);
  await page.goBack({ timeout: 15_000 });
  await expect(page).toHaveURL(expectedURL);
}
test("REAL_PG forms same-tab store history boundary", async ({ browser }) => {
  const { page, context } = await signed(browser);
  try {
    await step(
      page,
      "store history setup",
      "selectOption",
      "stable alternate-members then original-members history entries",
      async () => {
        await page.getByTestId("shell-store-selector").selectOption(alt);
        await expect(page).toHaveURL(list("en", alt));
        await expect(page.getByTestId("shell-store-selector")).toHaveValue(alt);
        await page.getByTestId("shell-store-selector").selectOption(store);
        await expect(page).toHaveURL(list("en"));
        await expect(page.getByTestId("shell-store-selector")).toHaveValue(store);
      },
    );
    const committed = await heldCreate(page, "M7_C_STORE_PRIVATE");
    await step(
      page,
      "held same-tab store boundary",
      "page.goBack",
      "one Back reaches alternate members and unmounts old create form before release",
      async () => {
        await nativeBack(page, list("en", alt));
        await expect(page.getByTestId("shell-store-selector")).toHaveValue(alt);
        await expect(page.getByTestId("members-list")).toBeVisible();
        await expect(page.getByTestId("member-form")).toHaveCount(0);
        await expect(page.getByTestId("member-create-open")).toHaveCount(0);
        await expect(page.locator('[name="name"]')).toHaveCount(0);
      },
    );
    await state("", "other_store");
    await ctl("forms-release");
    const after = await state("", "released");
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
    expect(after.altMembers).toBe(0);
    expect(after.altEntries).toBe(0);
    expect(after.altCommands).toBe(0);
    await expect(
      page.getByText("M7_C_STORE_PRIVATE", { exact: true }),
    ).toHaveCount(0);
  } finally {
    await ctl("forms-release");
    await context.close();
  }
});
test("REAL_PG forms same-tab member history boundary", async ({ browser }) => {
  const { page, context } = await signed(browser);
  try {
    await expect(page).toHaveURL(list("en"));
    const b = await create(page, "en", "M7_C_MEMBER_B");
    await expect(page).toHaveURL(`${origin}/en/members/${b}?store=${store}`);
    await page.getByTestId("member-back").click();
    await expect(page).toHaveURL(list("en"));
    const a = await create(page, "en", "M7_C_MEMBER_A");
    await expect(page).toHaveURL(`${origin}/en/members/${a}?store=${store}`);
    const beforeA = await state(a, "A_before"),
      beforeB = await state(b, "B_before");
    await review(page, "en", "7");
    await ctl("fault", "forms-hold");
    await page.getByTestId("member-form-confirm").click();
    await expect.poll(async () => (await state()).held).toBe(1);
    const committed = await state(a, "committed");
    expect(committed.balance).toBe(beforeA.balance + 7);
    await step(
      page,
      "held same-tab member boundary",
      "page.goBack twice",
      "A to list to B; B remains B before old A response release",
      async () => {
        await nativeBack(page, list("en"));
        await expect(page.getByTestId("members-list")).toBeVisible();
        await nativeBack(
          page,
          `${origin}/en/members/${b}?store=${store}`,
        );
        await expect(
          page.getByTestId("member-detail").locator("strong"),
        ).toHaveText("0");
        await expect(page.getByTestId("member-detail")).toContainText(
          "M7_C_MEMBER_B",
        );
      },
    );
    const afterB = await state(b, "B_after");
    expect(afterB.balance).toBe(beforeB.balance);
    expect(afterB.memberEntries).toBe(beforeB.memberEntries);
    await ctl("forms-release");
    const released = await state(a, "released");
    expect(released.commands).toBe(committed.commands);
    expect(released.writeRequests).toBe(committed.writeRequests);
    await expect(
      page.getByTestId("member-detail").locator("strong"),
    ).toHaveText("0");
  } finally {
    await ctl("forms-release");
    await context.close();
  }
});
test("REAL_PG forms reauthenticated session epoch", async ({ browser }) => {
  const { page, context } = await signed(browser);
  try {
    const previous = (await context.cookies()).find(
      (c) => c.name === "__Host-commerce_session",
    )?.value;
    const committed = await heldCreate(page, "M7_C_SESSION_PRIVATE");
    const other = await context.newPage();
    await other.goto(list("en"));
    await step(
      other,
      "new authenticated session",
      "click logout/login",
      "same actor receives a genuinely different session",
      async () => {
        await other
          .locator("summary")
          .filter({ hasText: shellCopy.en.account })
          .click();
        await other.getByTestId("workspace-sign-out").click();
        await other
          .getByRole("button", { name: "Sign in with identity service" })
          .click();
        await expect(other.getByTestId("shell-store-selector")).toBeVisible();
        const current = (await context.cookies()).find(
          (c) => c.name === "__Host-commerce_session",
        )?.value;
        sessionChanged = !!previous && !!current && previous !== current;
        expect(sessionChanged).toBe(true); // Boolean only; never log/export cookie or its hash.
      },
    );
    await expect(page.getByTestId("member-form")).toHaveCount(0);
    await state("", "new_session");
    await ctl("forms-release");
    const after = await state("", "released");
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
    await expect(page.locator('[name="name"]')).toHaveCount(0);
    await expect(
      page.getByText("M7_C_SESSION_PRIVATE", { exact: true }),
    ).toHaveCount(0);
  } finally {
    await ctl("forms-release");
    await context.close();
  }
});
test("REAL_PG forms erased before open", async ({ browser }) => {
  const { page, context } = await signed(browser);
  try {
    const a = await create(page, "en", "M7_C_ERASE_BEFORE");
    await ctl("forms-erase", a);
    const erased = await state(a, "erase_fixture");
    await page.reload();
    await expect(page.getByTestId("member-detail")).toContainText(
      membersCopy.en.erased,
    );
    await step(
      page,
      "erased adjustment disabled",
      "click",
      "real erased reason with no write",
      async () => {
        const open = page.getByTestId("member-adjust-open");
        await expect(open).toHaveCount(1);
        await expect(open).toBeVisible();
        await expect(open).toHaveAttribute("aria-disabled", "true");
        await expect(open).toHaveAttribute("title", memberFormCopy.en.erased);
        await expect(open).toHaveAttribute("aria-describedby", /\S+/);
        const reasonId = await open.getAttribute("aria-describedby");
        const reason = page.locator(`[id=${JSON.stringify(reasonId)}]`);
        await expect(reason).toHaveCount(1);
        await expect(reason).toHaveText(memberFormCopy.en.erased);
        // DS Button intentionally receives blocked clicks to reveal its reason.
        // Reuse ui-gallery's native pointer pattern: measure only, then real input.
        await open.scrollIntoViewIfNeeded();
        const box = await open.boundingBox(); // [READ/MEASURE] fresh viewport CSS pixels.
        expect(box).not.toBeNull();
        if (!box) throw new Error("erased adjustment button has no visible box");
        expect(box.width).toBeGreaterThan(0);
        expect(box.height).toBeGreaterThan(0);
        await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
        await expect(reason).toHaveAttribute("role", "status");
        await expect(
          page.getByText(memberFormCopy.en.erased, { exact: true }),
        ).toBeVisible();
        await expect(dialog(page)).not.toBeVisible();
      },
    );
    const after = await state(a, "erased_disabled");
    expect(after.commands).toBe(erased.commands);
    expect(after.entries).toBe(erased.entries);
    expect(after.writeRequests).toBe(erased.writeRequests);
  } finally {
    await context.close();
  }
});
test("REAL_PG forms erased after review", async ({ browser }) => {
  const { page, context } = await signed(browser);
  try {
    const a = await create(page, "en", "M7_C_ERASE_AFTER");
    await review(page, "en", "5");
    await ctl("forms-erase", a);
    const erased = await state(a, "erase_fixture");
    await step(
      page,
      "erased review refusal",
      "click",
      "actual409 member_erased distinct from insufficient/unavailable",
      async () => {
        const response = page.waitForResponse(
          (r) =>
            r.request().method() === "POST" &&
            new URL(r.url()).pathname.endsWith(`/members/${a}/adjustments`),
        );
        await page.getByTestId("member-form-confirm").click();
        expect((await response).status()).toBe(409);
        await expect(dialog(page)).toHaveAttribute("data-step", "refused");
        await expect(dialog(page).getByRole("alert")).toHaveText(
          memberFormCopy.en.erased,
        );
      },
    );
    const after = await state(a, "refused");
    expect(after.commands).toBe(erased.commands);
    expect(after.entries).toBe(erased.entries);
    expect(after.balance).toBe(erased.balance);
  } finally {
    await context.close();
  }
});
test("REAL_PG forms unavailable member read state", async ({ browser }) => {
  const { page, context } = await signed(browser);
  try {
    const before = await state();
    await step(
      page,
      "unavailable member navigation",
      "browser navigation setup/read",
      "real foreign-member READ404, unavailable UI and zero POST",
      async () => {
        // Declared read-navigation setup only: no public control offers a foreign member; no API write substitutes for UI.
        const response = page.waitForResponse(
          (r) =>
            r.request().method() === "GET" &&
            new URL(r.url()).pathname ===
              `/api/stores/${store}/members/${foreignMember}`,
        );
        await page.goto(`${origin}/en/members/${foreignMember}?store=${store}`);
        expect((await response).status()).toBe(404);
        await expect(
          page.getByText(membersCopy.en.notFound, { exact: true }),
        ).toBeVisible();
        await expect(page.getByTestId("member-ledger")).toHaveCount(0);
        await expect(
          page.getByTestId("member-detail").locator("dl"),
        ).toHaveCount(0);
        const action = page.getByTestId("member-adjust-open");
        if (await action.count())
          await expect(action).toHaveAttribute("aria-disabled", "true");
      },
    );
    const after = await state("", "read404");
    expect(after.writeRequests).toBe(before.writeRequests);
    expect(after.members).toBe(before.members);
    expect(after.entries).toBe(before.entries);
    expect(after.commands).toBe(before.commands);
  } finally {
    await context.close();
  }
});

async function localizedCreateUnknown(browser: Browser, locale: Locale) {
  const { page, context } = await signed(browser, locale);
  const before = await state();
  try {
    await page.getByTestId("member-create-open").click();
    await fill(page, "name", "M7 synthetic dropped create");
    await ctl("fault", "forms-drop");
    await step(
      page,
      "create response lost",
      "click",
      "committed create becomes UNKNOWN",
      async () => {
        await page.getByTestId("member-form-submit").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
      },
    );
    await captureResponsive(page, locale, "CreateUnknown");
    const committed = await state("", "unknown");
    expect(committed.members).toBe(before.members + 1);
    expect(committed.commands).toBe(before.commands + 1);
    await step(
      page,
      "UNKNOWN read only",
      "click/keyboard",
      "refresh and Escape cannot resend or resolve intent",
      async () => {
        await page.getByTestId("member-form-refresh").click();
        await page.keyboard.press("Escape");
        await expect(dialog(page)).toBeVisible();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
        await expect(page.getByTestId("member-form-submit")).toHaveCount(0);
      },
    );
    await state("", "refreshed");
    await state("", "escape_blocked");
    await expect(page.getByTestId("member-form-close")).toHaveCount(0);
    await expect(page.getByTestId("member-form-cancel")).toHaveCount(0);
    const after = await state();
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
  } finally {
    await context.close();
  }
}
test("REAL_PG forms create pending double click then committed UNKNOWN never resends zh-TW", async ({ browser }) => {
  await localizedCreateUnknown(browser, "zh-TW");
});
test("REAL_PG forms create pending double click then committed UNKNOWN never resends zh-CN", async ({ browser }) => {
  await localizedCreateUnknown(browser, "zh-CN");
});

async function localizedAdjustUnknown(browser: Browser, locale: Locale) {
  const { page, context } = await signed(browser, locale);
  try {
    const id = await create(page, locale, `M7_UNKNOWN_ADJUST_${locale}`);
    await review(page, locale, "5");
    const before = await state(id);
    await ctl("fault", "forms-drop");
    await page.getByTestId("member-form-confirm").click();
    await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
    await captureResponsive(page, locale, "Unknown");
    const committed = await state(id, "unknown");
    expect(committed.balance).toBe(5);
    expect(committed.sum).toBe(5);
    expect(committed.memberEntries).toBe(1);
    expect(committed.commands).toBe(before.commands + 1);
    await step(
      page,
      "adjust UNKNOWN refresh",
      "click",
      "read refresh keeps unresolved state and sends no POST",
      async () => {
        await page.getByTestId("member-form-refresh").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
      },
    );
    await state(id, "refreshed");
    await page.keyboard.press("Escape");
    await expect(dialog(page)).toHaveAttribute("data-step", "unknown");
    await state(id, "escape_blocked");
    await expect(page.getByTestId("member-form-close")).toHaveCount(0);
    await expect(page.getByTestId("member-form-cancel")).toHaveCount(0);
    const after = await state(id);
    expect(after.commands).toBe(committed.commands);
    expect(after.writeRequests).toBe(committed.writeRequests);
  } finally {
    await context.close();
  }
}
test("REAL_PG forms adjust committed UNKNOWN reads SQL effect exactly once zh-TW", async ({ browser }) => {
  await localizedAdjustUnknown(browser, "zh-TW");
});
test("REAL_PG forms adjust committed UNKNOWN reads SQL effect exactly once zh-CN", async ({ browser }) => {
  await localizedAdjustUnknown(browser, "zh-CN");
});

async function localizedRaceRefused(browser: Browser, locale: Locale) {
  const { page, context } = await signed(browser, locale);
  try {
    const id = await create(page, locale, `M7_RACE_${locale}`);
    await review(page, locale, "50");
    await confirm(page, 50);
    await review(page, locale, "30", true);
    await ctl("forms-race", id);
    const before = await state(id, "race_fixture");
    await step(
      page,
      "server concurrent deduction refusal",
      "click",
      "actual409 visible and no second deduction",
      async () => {
        await page.getByTestId("member-form-confirm").click();
        await expect(dialog(page)).toHaveAttribute("data-step", "refused");
        await expect(dialog(page).getByRole("alert")).toContainText(
          memberFormCopy[locale].insufficient,
        );
      },
    );
    await captureResponsive(page, locale, "Refused");
    const after = await state(id, "refused");
    expect(after.commands).toBe(before.commands);
    expect(after.memberEntries).toBe(before.memberEntries);
    expect(after.balance).toBe(0);
    await step(
      page,
      "refusal back",
      "click",
      "draft can be corrected",
      async () => {
        await page.getByTestId("member-form-back").click();
        await expect(page.locator('[name="amount"]')).toHaveValue("30");
      },
    );
    await page.getByTestId("member-form-submit").click();
    await page.getByTestId("member-form-confirm").click();
    await expect(dialog(page)).toHaveAttribute("data-step", "refused");
    await step(
      page,
      "refusal close",
      "click",
      "modal closes and opener focused",
      async () => {
        await page.getByTestId("member-form-close").click();
        await expect(dialog(page)).not.toBeVisible();
        await expect(page.getByTestId("member-adjust-open")).toBeFocused();
      },
    );
  } finally {
    await context.close();
  }
}
test("REAL_PG forms genuine server race refusal and correction controls zh-TW", async ({ browser }) => {
  await localizedRaceRefused(browser, "zh-TW");
});
test("REAL_PG forms genuine server race refusal and correction controls zh-CN", async ({ browser }) => {
  await localizedRaceRefused(browser, "zh-CN");
});

for (const locale of ["en", "zh-TW", "zh-CN"] as const) {
  test(`REAL_PG forms genuine create permission refusal ${locale}`, async ({ browser }) => {
    const { page, context } = await signed(browser, locale);
    try {
      await page.getByTestId("member-create-open").click();
      await fill(page, "name", "M7 synthetic permission refusal");
      const before = await state();
      // Fixture changes only the existing synthetic grant after the real form opens.
      await ctl("forms-points-grant", "deny");
      await step(page, "create permission refusal", "click", "genuine POST403 and localized refused UI", async () => {
        const response = page.waitForResponse(r =>
          r.request().method() === "POST" &&
          new URL(r.url()).pathname === `/api/stores/${store}/members`);
        await page.getByTestId("member-form-submit").click();
        const refused = await response;
        expect(refused.status()).toBe(403);
        expect(await refused.text()).toContain("forbidden");
        await expect(dialog(page)).toHaveAttribute("data-step", "refused");
        await expect(dialog(page).getByRole("alert")).toContainText(memberFormCopy[locale].forbidden);
      });
      await captureResponsive(page, locale, "CreateRefused");
      const after = await state("", "refused");
      for (const key of ["members", "entries", "commands", "totalBalance", "erasedMembers"])
        expect(after[key], key).toBe(before[key]);
      expect(after.writeRequests).toBe(before.writeRequests + 1);
    } finally {
      try {
        // Handler returns204 only after exact captured grant state is restored.
        await ctl("forms-points-grant", "restore");
        await step(page, "restored read access", "reload", "production member list remains readable", async () => {
          await page.reload();
          await expect(page.getByTestId("members-list")).toBeVisible();
        });
      } finally {
        await context.close();
      }
    }
  });
}
