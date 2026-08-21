import { useModal } from "../../providers/modalContext";
import { RegisterSection } from "../forms/RegisterSection";
import Button from "../objects/Button.tsx";

export function RegisterModal() {
  const { closeModal, openModal } = useModal();

  return (
    <div className="space-y-2 p-6">
      <h2 className="mb-4 text-lg font-semibold">Register</h2>
      <RegisterSection onClose={closeModal} />
      <div>
        <span>Already have an account? </span>
        <Button variant="tertiary" onClick={() => openModal("login")}>
          Log in
        </Button>
      </div>
    </div>
  );
}
