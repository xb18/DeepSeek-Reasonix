import { describe, expect, it } from "vitest";
import type { Item } from "../state/session_types";
import { repeatedSpeakers, sameMembers, transcriptRows } from "./turnrows";

const me = (id: string, text: string, pending?: boolean): Item => ({ t: "user", id, text, pending });
const said = (id: string, text: string): Item => ({ t: "say", id, text, done: true });
const ran = (id: string): Item => ({ t: "tool", id, tool: { id, name: "bash" } as never, running: false, children: [] });

const shape = (items: Item[]) =>
  transcriptRows(items).map((r) => ("item" in r ? `${r.item.t}${r.activity ? `+${r.activity.length}` : ""}` : `work:${r.activity.length}`));

describe("what a turn looks like once it is laid out", () => {
  it("files each run of work under the sentence it followed", () => {
    expect(shape([me("u1", "go"), said("s1", "on it"), ran("t1"), ran("t2"), said("s2", "done")]))
      .toEqual(["user", "say+2", "say"]);
  });

  // A line typed into a running turn used to sit here from the moment it was
  // typed. It split the turn at that point, so every tool call that followed
  // was filed under an instruction the model had not read — the transcript
  // reporting work as done about a line that had not been sent.
  it("leaves a queued line out rather than splitting the turn it was typed into", () => {
    const withQueued = [me("u1", "go"), said("s1", "on it"), me("u2", "also this", true), ran("t1"), said("s2", "done")];
    expect(shape(withQueued)).toEqual(shape([me("u1", "go"), said("s1", "on it"), ran("t1"), said("s2", "done")]));
  });

  it("draws the line as soon as the kernel has taken it", () => {
    expect(shape([me("u1", "go"), said("s1", "on it"), me("u2", "also this")])).toEqual(["user", "say", "user"]);
  });

  // A model that calls tools with no answer yet still streams a message, often
  // with a blank thought. That message draws nothing, so work filed under it
  // would draw nothing either.
  it("files work under the sentence that is drawn, not a blank one before it", () => {
    const blank: Item = { t: "say", id: "s0", text: "", reasoning: "\n", done: true };
    expect(shape([me("u1", "go"), blank, ran("t1"), ran("t2"), said("s1", "found it"), ran("t3"), said("s2", "done")]))
      .toEqual(["user", "say", "say+3", "say"]);
  });

  it("keeps work under the model that did it while no sentence has come yet", () => {
    const blank: Item = { t: "say", id: "s0", text: "", reasoning: "\n", done: true };
    expect(shape([me("u1", "go"), blank, ran("t1"), ran("t2")])).toEqual(["user", "say+2"]);
  });
});

describe("which sentences repeat a speaker already on screen", () => {
  const by = (id: string, model?: string): Item => ({ t: "say", id, text: id, done: true, model });
  const names = (items: Item[]) => [...repeatedSpeakers(items)];

  it("marks every sentence after the first until the user speaks", () => {
    expect(names([me("u1", "a"), by("s1"), ran("t1"), by("s2"), by("s3"), me("u2", "b"), by("s4")])).toEqual(["s2", "s3"]);
  });

  it("keeps the header where the model changes", () => {
    expect(names([me("u1", "a"), by("s1", "m1"), by("s2", "m2"), by("s3", "m2")])).toEqual(["s3"]);
  });

  it("does not let a queued line or an empty sentence reset or stand in", () => {
    expect(names([me("u1", "a"), by("s1"), me("q", "later", true), { t: "say", id: "e", text: "", done: false }, by("s2")])).toEqual(["s2"]);
  });
});

describe("keeping a set when nothing in it changed", () => {
  it("compares members, not references", () => {
    expect(sameMembers(new Set(["a", "b"]), new Set(["b", "a"]))).toBe(true);
    expect(sameMembers(new Set(["a"]), new Set(["a", "b"]))).toBe(false);
    expect(sameMembers(new Set(["a", "c"]), new Set(["a", "b"]))).toBe(false);
  });
});
