import { describe, expect, it } from "vitest";

import { MANIFEST_API_VERSION, UI_API_MAJOR } from "./index";

describe("contract versions", () => {
  it("exports the frozen v1 identifiers", () => {
    expect(MANIFEST_API_VERSION).toBe("relayward.plugin/v1");
    expect(UI_API_MAJOR).toBe(1);
  });
});
