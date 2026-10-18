// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import "./testkit";
import { ToolList } from "./ToolList";
import { MockPort } from "../port/mock";
import type { AgentPort } from "../port/port";

afterEach(cleanup);

describe("the tool list", () => {
  it("shows every tool with a switch, saves the opposite of what a switch shows, and says when it takes effect", async () => {
    const p = new MockPort() as unknown as AgentPort;
    const save = vi.spyOn(p, "saveToolDisabled");
    render(<ToolList port={p} />);
    const sw = await screen.findByRole("switch", { name: "web_fetch" });
    expect(sw.getAttribute("aria-checked")).toBe("true");
    expect(screen.getByText(/新开的对话生效/)).toBeTruthy();
    await userEvent.click(sw);
    expect(save).toHaveBeenCalledWith("web_fetch", true);
    await waitFor(() => expect(sw.getAttribute("aria-checked")).toBe("false"));
    await userEvent.click(sw);
    expect(save).toHaveBeenLastCalledWith("web_fetch", false);
  });

  it("gives a tool the host needs no switch and says what needs it", async () => {
    const p = new MockPort() as unknown as AgentPort;
    render(<ToolList port={p} />);
    await screen.findByRole("switch", { name: "web_fetch" });
    expect(screen.queryByRole("switch", { name: "update_goal" })).toBeNull();
    expect(screen.getByText(/目标模式需要它/)).toBeTruthy();
  });

  it("keeps the switch where it was when the save is refused", async () => {
    const p = new MockPort() as unknown as AgentPort;
    p.saveToolDisabled = async () => {
      throw new Error("disk full");
    };
    render(<ToolList port={p} />);
    const sw = await screen.findByRole("switch", { name: "web_fetch" });
    await userEvent.click(sw);
    await screen.findByText(/disk full/);
    expect(sw.getAttribute("aria-checked")).toBe("true");
  });

  it("says so when the list cannot be read", async () => {
    const p = new MockPort() as unknown as AgentPort;
    p.toolSettings = async () => {
      throw new Error("offline");
    };
    render(<ToolList port={p} />);
    expect(await screen.findByText("无法读取工具列表。")).toBeTruthy();
  });
});
