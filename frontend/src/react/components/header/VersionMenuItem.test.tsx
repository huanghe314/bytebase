import type { ReactElement } from "react";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { beforeEach, describe, expect, test, vi } from "vitest";

(
  globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }
).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  serverInfo: {
    version: "1.0.0",
    gitCommit: "backend123",
    saas: false,
  },
  closeMenu: vi.fn(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock("@/react/hooks/useAppState", () => ({
  useServerInfo: () => mocks.serverInfo,
}));

let VersionMenuItem: typeof import("./VersionMenuItem").VersionMenuItem;

const renderIntoContainer = (element: ReactElement) => {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  return {
    container,
    render: () => {
      act(() => {
        root.render(element);
      });
    },
    unmount: () =>
      act(() => {
        root.unmount();
        container.remove();
      }),
  };
};

beforeEach(async () => {
  vi.clearAllMocks();
  mocks.serverInfo = {
    version: "1.0.0",
    gitCommit: "backend123",
    saas: false,
  };
  ({ VersionMenuItem } = await import("./VersionMenuItem"));
});

describe("VersionMenuItem", () => {
  test("shows version and git hashes in non-SaaS mode", () => {
    const { container, render, unmount } = renderIntoContainer(
      <VersionMenuItem onCloseMenu={mocks.closeMenu} />
    );
    render();

    expect(container.textContent).toContain("v1.0.0");
    expect(container.textContent).toContain("BE Git hash");
    expect(container.textContent).toContain("FE Git hash");
    unmount();
  });

  test("hides version info in SaaS/cloud mode", () => {
    mocks.serverInfo = {
      version: "1.0.0",
      gitCommit: "backend123",
      saas: true,
    };
    const { container, render, unmount } = renderIntoContainer(
      <VersionMenuItem onCloseMenu={mocks.closeMenu} />
    );
    render();

    expect(container.textContent).not.toContain("v1.0.0");
    expect(container.textContent).not.toContain("BE Git hash");
    expect(container.textContent).not.toContain("FE Git hash");
    unmount();
  });
});
