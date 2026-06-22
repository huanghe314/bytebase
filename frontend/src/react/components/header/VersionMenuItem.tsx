import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import {
  useServerInfo,
} from "@/react/hooks/useAppState";

export function VersionMenuItem({ onCloseMenu }: { onCloseMenu: () => void }) {
  const { t } = useTranslation();
  const serverInfo = useServerInfo();

  const version = serverInfo?.version ?? "";
  const gitCommitBE = serverInfo?.gitCommit || "unknown";
  const gitCommitFE = import.meta.env.GIT_COMMIT || "unknown";

  const formattedVersion = useMemo(() => {
    if (version && version.split(".").length === 3) {
      return `v${version}`;
    }
    return version || "unknown";
  }, [version]);

  return (
    <>
      <div className="px-3 py-2">
        {!serverInfo?.saas ? (
          <>
            <div className="flex w-full items-center justify-between gap-x-2 rounded-sm px-0 py-1 text-left text-sm text-control">
              <span className="flex items-center gap-x-2">
                {formattedVersion}
              </span>
            </div>

            <div className="mt-1 text-xs text-control-light">
              <div>BE Git hash: {gitCommitBE.slice(0, 7)}</div>
              <div>FE Git hash: {gitCommitFE.slice(0, 7)}</div>
            </div>
          </>
        ) : null}
      </div>
    </>
  );
}
