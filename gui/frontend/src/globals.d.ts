export {};

declare global {
  interface Window {
    go?: { main?: { App?: Record<string, (...args: unknown[]) => Promise<unknown>> } };
    runtime?: {
      EventsOn: (name: string, callback: (data: unknown) => void) => () => void;
    };
  }
}
