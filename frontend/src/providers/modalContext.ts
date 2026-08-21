import { createContext, useContext } from "react";

export type DialogType = "login" | "register" | "imageUpload" | "deleteAccount" | null;

// Extra data a caller can hand to a specific modal when opening it. Only
// "imageUpload" needs one today - it hands back the file the user picked
// once they confirm, so e.g. Avatar/Profile can show the new preview.
export interface ImageUploadModalOptions {
  onComplete?: (file: File) => void;
}

export interface ModalContextValue {
  activeModal: DialogType;
  chatOpen: boolean;
  imageUploadOptions: ImageUploadModalOptions | null;
  openModal: (modal: DialogType, options?: ImageUploadModalOptions) => void;
  closeModal: () => void;
  openChat: () => void;
  closeChat: () => void;
}

export const ModalContext = createContext<ModalContextValue | null>(null);

export function useModal() {
  const ctx = useContext(ModalContext);
  if (!ctx) throw new Error("useModal must be used within ModalProvider");
  return ctx;
}
