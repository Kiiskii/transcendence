import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Form } from "./Form";
import { FormField } from "./FormField";
import { registerSchema, type RegisterFormSchema } from "../../schemas/register";
import Button from "../objects/Button.tsx";
import { useAuth } from "../../hooks/useAuth";
import { isApiError } from "../../api/client";

export function RegisterSection({ onClose }: { onClose: () => void }) {
  const { signup } = useAuth();
  const form = useForm<RegisterFormSchema>({
    resolver: zodResolver(registerSchema),
    mode: "onBlur",
  });
  const {
    formState: { errors, isValid, isSubmitting },
  } = form;

  const handleSubmit = async (data: RegisterFormSchema) => {
    form.clearErrors("root");
    try {
      // The backend signs the user in as part of signing up, so nothing more
      // to do on success than close.
      await signup(data.username, data.email, data.password);
      onClose();
    } catch (err) {
      if (isApiError(err)) {
        // A clash or validation miss names its field, so the message lands
        // under the offending input like any client-side zod error would.
        const details = err.details ?? {};
        if (details.username) form.setError("username", { message: details.username });
        else if (details.email) form.setError("email", { message: details.email });
        else form.setError("root", { message: err.message });
      } else {
        form.setError("root", { message: "Something went wrong. Please try again." });
      }
    }
  };

  return (
    <Form form={form} onSubmit={handleSubmit} isEditing={true}>
      <div className="space-y-4">
        <div className="space-y-2">
          <FormField label="Username" name="username" validateOnChange />
          <FormField label="Email" name="email" validateOnChange />
          <FormField label="Password" name="password" type="password" validateOnChange />
        </div>
        {errors.root?.message && <p className="text-berry-500 text-sm">{errors.root.message}</p>}
        <div className="flex flex-row gap-2">
          <Button variant="primary" type="submit" disabled={!isValid || isSubmitting}>
            {isSubmitting ? "Registering…" : "Register"}
          </Button>
          <Button variant="secondary" type="button" onClick={onClose}>
            Cancel
          </Button>
        </div>
      </div>
    </Form>
  );
}
