// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "./testkit";
import { SubagentSteerLimits } from "./SubagentSteerLimits";
import type { AgentPort } from "../port/port";
import type { SubagentSteerSettings } from "../port/boundary";

afterEach(cleanup);

const base: SubagentSteerSettings = { maxBytes: 16384, perMinute: 30, defaultMaxBytes: 16384, defaultPerMinute: 30, path: "/u/config.toml" };

function portWith(initial = base) {
  const save = vi.fn(async (maxBytes: number, perMinute: number) => ({ ...initial, maxBytes, perMinute }));
  return { subagentSteerSettings: vi.fn(async () => initial), saveSubagentSteerSettings: save } as unknown as AgentPort & { saveSubagentSteerSettings: typeof save };
}

const size = () => screen.findByRole("textbox", { name: "单条指示的字节上限" }) as Promise<HTMLInputElement>;
const rate = () => screen.findByRole("textbox", { name: "每分钟条数上限" }) as Promise<HTMLInputElement>;

describe("the limits on guidance sent to a running sub-agent", () => {
  it("shows the two named defaults and says zero lifts a limit", async () => {
    render(<SubagentSteerLimits port={portWith()} />);
    expect((await size()).value).toBe("16384");
    expect((await rate()).value).toBe("30");
    expect(screen.getAllByText(/0/).length).toBeGreaterThan(0);
  });

  it("writes both numbers once, on Enter, keeping the one it did not touch", async () => {
    const port = portWith();
    render(<SubagentSteerLimits port={port} />);
    const field = await size();
    await userEvent.clear(field);
    await userEvent.type(field, "4096{Enter}");
    expect(port.saveSubagentSteerSettings).toHaveBeenCalledTimes(1);
    expect(port.saveSubagentSteerSettings).toHaveBeenCalledWith(4096, 30);
  });

  it("lifts a limit with zero and writes nothing for a value it did not change", async () => {
    const port = portWith();
    render(<SubagentSteerLimits port={port} />);
    const field = await rate();
    await userEvent.clear(field);
    await userEvent.type(field, "0{Enter}");
    expect(port.saveSubagentSteerSettings).toHaveBeenLastCalledWith(16384, 0);
    port.saveSubagentSteerSettings.mockClear();
    const again = await size();
    again.focus();
    again.blur();
    expect(port.saveSubagentSteerSettings).not.toHaveBeenCalled();
  });

  it("takes digits only, so a number that is not one is never sent", async () => {
    const port = portWith();
    render(<SubagentSteerLimits port={port} />);
    const field = await size();
    await userEvent.clear(field);
    await userEvent.type(field, "12ab-3{Enter}");
    expect(port.saveSubagentSteerSettings).toHaveBeenCalledWith(123, 30);
  });

  it("shows why a save failed and keeps what the person typed", async () => {
    const port = portWith();
    port.saveSubagentSteerSettings.mockRejectedValueOnce(new Error("disk"));
    render(<SubagentSteerLimits port={port} />);
    const field = await size();
    await userEvent.clear(field);
    await userEvent.type(field, "99{Enter}");
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect((await size()).value).toBe("99");
  });
});
