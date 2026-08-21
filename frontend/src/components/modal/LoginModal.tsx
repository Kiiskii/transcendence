import { useModal } from "../../providers/modalContext";
import { LoginSection } from "../forms/LoginSection";
import Button from "../objects/Button.tsx";

export function LoginModal() {
  const { closeModal, openModal } = useModal();

  return (
    <div className="space-y-2 p-6">
      <h2 className="mb-4 text-lg font-semibold">Log in</h2>
      <LoginSection onClose={closeModal} />
      <div className="">
        <span>Not registered yet? </span>
        <Button variant="tertiary" onClick={() => openModal("register")}>
          Register
        </Button>
      </div>
    </div>
  );
}
