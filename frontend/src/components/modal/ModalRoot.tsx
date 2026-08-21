import { useModal } from "../../providers/modalContext";
import { type DialogType } from "../../providers/modalContext";
import { Modal } from "./Modal";
import { LoginModal } from "./LoginModal";
import { RegisterModal } from "./RegisterModal";
import { ImageUploadModal } from "./ImageUploadModal";
import { DeleteAccountModal } from "./DeleteAccountModal";

const sizeByModal: Record<NonNullable<DialogType>, string> = {
  login: "max-w-sm",
  register: "max-w-sm",
  imageUpload: "max-w-fit",
  deleteAccount: "max-w-sm",
};

export function ModalRoot() {
  const { activeModal, closeModal } = useModal();
  if (!activeModal) return null;

  return (
    <Modal onClose={closeModal} variant="dialog" className={sizeByModal[activeModal]}>
      {activeModal === "login" && <LoginModal />}
      {activeModal === "register" && <RegisterModal />}
      {activeModal === "imageUpload" && <ImageUploadModal />}
      {activeModal === "deleteAccount" && <DeleteAccountModal />}
    </Modal>
  );
}
