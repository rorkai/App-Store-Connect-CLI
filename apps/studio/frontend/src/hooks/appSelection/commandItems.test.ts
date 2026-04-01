import { describe, expect, it } from "vitest";

import { parseCommandItems } from "./commandItems";

describe("parseCommandItems", () => {
  it("parses JSON:API collections", () => {
    expect(parseCommandItems(JSON.stringify({
      data: [
        { id: "build-1", type: "builds", attributes: { version: "1.0", processingState: "VALID" } },
      ],
    }))).toEqual([
      { id: "build-1", type: "builds", version: "1.0", processingState: "VALID" },
    ]);
  });

  it("parses single JSON:API resources", () => {
    expect(parseCommandItems(JSON.stringify({
      data: { id: "age-rating-1", type: "ageRatings", attributes: { appStoreAgeRating: "17_PLUS" } },
    }))).toEqual([
      { id: "age-rating-1", type: "ageRatings", appStoreAgeRating: "17_PLUS" },
    ]);
  });

  it("prefers top-level object arrays for generic command responses", () => {
    expect(parseCommandItems(JSON.stringify({
      summary: { health: "warn", nextAction: "Fix credentials" },
      checks: [
        { name: "authentication", status: "warn", message: "auth doctor found 1 warning(s)" },
        { name: "api_access", status: "ok", message: "able to read apps list" },
      ],
      generatedAt: "2026-03-31T00:00:00Z",
    }))).toEqual([
      { id: "checks-1", name: "authentication", status: "warn", message: "auth doctor found 1 warning(s)" },
      { id: "checks-2", name: "api_access", status: "ok", message: "able to read apps list" },
    ]);
  });

  it("flattens scalar top-level objects when no item array exists", () => {
    expect(parseCommandItems(JSON.stringify({
      summary: { health: "ok", nextAction: "Ship it" },
      generatedAt: "2026-03-31T00:00:00Z",
    }))).toEqual([
      {
        "summary.health": "ok",
        "summary.nextAction": "Ship it",
        generatedAt: "2026-03-31T00:00:00Z",
      },
    ]);
  });
});
