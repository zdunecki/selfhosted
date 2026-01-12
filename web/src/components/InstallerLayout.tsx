import React from "react";
import { Check, ChevronRight, ChevronLeft } from "lucide-react";

export interface Step {
  id: string;
  title: string;
  description?: string;
}

interface InstallerLayoutProps {
  steps: Step[];
  currentStepIndex: number;
  appName: string;
  appLogo?: string;
  canNext?: boolean;
  isNextLoading?: boolean;
  nextLabel?: string;
  onNext?: () => void;
  onBack?: () => void;
  children: React.ReactNode;
}

export function InstallerLayout({
  steps,
  currentStepIndex,
  appName,
  appLogo,
  canNext = true,
  isNextLoading = false,
  nextLabel = "Continue",
  onNext,
  onBack,
  children,
}: InstallerLayoutProps) {
  return (
    <div className="bg-zinc-50 text-zinc-900 flex items-center justify-center p-4 font-sans selection:bg-blue-100 selection:text-blue-900">
      {/* Main Window */}
      <div className="w-full max-w-5xl h-[800px] bg-white rounded-2xl shadow-2xl border border-zinc-200 flex overflow-hidden relative">
        {/* Sidebar */}
        <div className="w-64 bg-zinc-50/50 border-r border-zinc-200 flex flex-col pt-8 pb-4 relative z-10 backdrop-blur-xl">
          {/* Window Controls (Decorative) */}
          <div className="absolute top-4 left-4 flex gap-2">
            <div className="w-3 h-3 rounded-full bg-red-400/80 border border-red-500/20"></div>
            <div className="w-3 h-3 rounded-full bg-yellow-400/80 border border-yellow-500/20"></div>
            <div className="w-3 h-3 rounded-full bg-green-400/80 border border-green-500/20"></div>
          </div>

          <div className="px-6 mt-8 mb-8">
            <div className="mb-3 flex items-center gap-3">
              {appLogo && (
                <img
                  src={appLogo}
                  alt={appName || ""}
                  className="w-8 h-8 object-contain"
                />
              )}
              {appName && (
                <h2 className="font-semibold text-zinc-900 text-sm">
                  {appName}
                </h2>
              )}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto px-4 space-y-1">
            {steps.map((step, index) => {
              const isCompleted = index < currentStepIndex;
              const isCurrent = index === currentStepIndex;

              return (
                <div
                  key={step.id}
                  className={`
                                        flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-all duration-200
                                        ${
                                          isCurrent
                                            ? "bg-white text-zinc-900 font-medium shadow-sm ring-1 ring-zinc-200"
                                            : ""
                                        }
                                        ${isCompleted ? "text-zinc-500" : ""}
                                        ${
                                          !isCurrent && !isCompleted
                                            ? "text-zinc-500 hover:bg-zinc-100/50"
                                            : ""
                                        }
                                    `}
                >
                  <div
                    className={`
                                        w-5 h-5 rounded-full flex items-center justify-center text-[10px] border transition-colors
                                        ${
                                          isCompleted
                                            ? "bg-blue-600 border-blue-600 text-white"
                                            : ""
                                        }
                                        ${
                                          isCurrent
                                            ? "bg-blue-600 border-blue-600 text-white"
                                            : ""
                                        }
                                        ${
                                          !isCurrent && !isCompleted
                                            ? "border-zinc-300 text-zinc-400 bg-white"
                                            : ""
                                        }
                                    `}
                  >
                    {isCompleted ? (
                      <Check size={12} strokeWidth={3} />
                    ) : (
                      index + 1
                    )}
                  </div>
                  <span className="truncate">{step.title}</span>
                </div>
              );
            })}
          </div>
          <div className="overflow-y-auto font-mono text-xs px-4 space-y-2">
            <div className="flex flex-col justify-between gap-3">
              <p className="text-zinc-600">
                Show your support! Star us on GitHub ⭐️{" "}
                <a
                  href="https://github.com/zdunecki/selfhosted/issues"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-500 hover:text-blue-600"
                />
              </p>

              <div className="shrink-0">
                <iframe
                  title="GitHub Stargazers"
                  src="https://ghbtns.com/github-btn.html?user=zdunecki&repo=selfhosted&type=star&count=true&size=small"
                  frameBorder="0"
                  scrolling="0"
                  width="120"
                  height="20"
                />
              </div>
              <p className="text-zinc-600 text-[9px] italic text-center">
                SelfHosted does not store and credentials, read more{" "}
                <a
                  href="https://github.com/zdunecki/selfhosted/blob/main/README.md"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-500 hover:text-blue-600"
                >
                  here
                </a>
                .
              </p>
            </div>
          </div>
        </div>

        {/* Main Content Area */}
        <div className="flex-1 flex flex-col min-w-0 min-h-0 bg-white">
          <div className="flex-1 min-h-0 overflow-y-auto p-8 relative">
            <div className="max-w-3xl mx-auto">{children}</div>
          </div>

          {/* Footer Controls */}
          <div className="px-8 py-5 border-t border-zinc-100 bg-white/80 backdrop-blur-sm flex items-center justify-between">
            <div>
              {onBack && currentStepIndex > 0 && (
                <button
                  onClick={onBack}
                  className="px-4 py-2 rounded-lg text-sm font-medium text-zinc-500 hover:text-zinc-900 hover:bg-zinc-100 transition-colors flex items-center gap-2"
                >
                  <ChevronLeft size={16} />
                  Back
                </button>
              )}
            </div>

            {onNext && (
              <button
                onClick={onNext}
                disabled={!canNext || isNextLoading}
                className={`
                                    px-6 py-2.5 rounded-lg text-sm font-medium transition-all duration-200 flex items-center gap-2 shadow-sm
                                    ${
                                      canNext && !isNextLoading
                                        ? "bg-zinc-800 text-white hover:bg-zinc-800 hover:shadow-md"
                                        : "bg-zinc-100 text-zinc-400 cursor-not-allowed"
                                    }
                                `}
              >
                {isNextLoading ? (
                  <div className="w-4 h-4 border-2 border-zinc-400 border-t-white rounded-full animate-spin" />
                ) : (
                  <>
                    {nextLabel}
                    <ChevronRight size={16} />
                  </>
                )}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
