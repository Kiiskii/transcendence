import "./index.css";

import React from "react";
import ReactDOM from "react-dom/client";

import { QueryClientProvider } from "@tanstack/react-query";

import AppRouter from "./routes";
import { queryClient } from "./lib/queryClient";
import { ErrorBoundary } from "./components/layout/ErrorBoundary";

import { ModalProvider } from "./providers/ModalProvider";
import { AuthProvider } from "./providers/AuthProvider";
import { ModalRoot } from "./components/modal/ModalRoot";
import { ChatRoot } from "./components/modal/ChatRoot";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <ModalProvider>
            <AppRouter />
            <ChatRoot />
            <ModalRoot />
          </ModalProvider>
        </AuthProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  </React.StrictMode>,
);
