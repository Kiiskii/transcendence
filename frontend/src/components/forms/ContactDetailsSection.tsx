import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Form } from "./Form";
import { FormField } from "./FormField";
import { contactDetailsSchema, type ContactDetailsFormValues } from "../../schemas/contactDetails";
import Button from "../objects/Button.tsx";
import { useEffect, useState } from "react";
import { useOwnProfile, useUpdateOwnProfile } from "../../api/profile";
import { isApiError } from "../../api/client";

export function ContactDetailsSection() {
  const [isEditing, setEditing] = useState(false);
  const { data: profile } = useOwnProfile();
  const update = useUpdateOwnProfile();

  const form = useForm<ContactDetailsFormValues>({
    resolver: zodResolver(contactDetailsSchema),
    mode: "onBlur",
  });
  const {
    formState: { errors, isValid, isSubmitting },
  } = form;

  // Seed the fields once the profile lands (and after any refetch), but never
  // clobber what the user is typing mid-edit.
  useEffect(() => {
    if (profile && !isEditing) {
      form.reset({
        firstname: profile.firstname ?? "",
        lastname: profile.lastname ?? "",
        phone_number: profile.phone_number ?? "",
        location: profile.location ?? "",
      });
    }
  }, [profile, isEditing, form]);

  const handleSubmit = async (data: ContactDetailsFormValues) => {
    form.clearErrors("root");
    try {
      await update.mutateAsync({
        firstname: data.firstname,
        lastname: data.lastname,
        phone_number: data.phone_number,
        location: data.location,
      });
      setEditing(false);
    } catch (err) {
      form.setError("root", {
        message: isApiError(err) ? err.message : "Something went wrong. Please try again.",
      });
    }
  };

  return (
    <Form form={form} onSubmit={handleSubmit} className="max-w-fit" isEditing={isEditing}>
      <div className="space-y-2">
        <div className="grid grid-cols-2 gap-4">
          <FormField label="First name" name="firstname" validateOnChange />
          <FormField label="Last name" name="lastname" />
          <FormField label="Phone" name="phone_number" type="tel" />
          <FormField label="City" name="location" validateOnChange />
        </div>
        {errors.root?.message && <p className="text-berry-500 text-sm">{errors.root.message}</p>}
        <div className="flex flex-row gap-2">
          {isEditing ? (
            <>
              <Button variant="primary" type="submit" disabled={!isValid || isSubmitting}>
                {isSubmitting ? "Saving…" : "Save"}
              </Button>
              <Button
                variant="secondary"
                onClick={() => {
                  form.reset();
                  setEditing(false);
                }}
              >
                Cancel
              </Button>
            </>
          ) : (
            <Button variant="primary" type="button" onClick={() => setEditing(true)}>
              Edit Details
            </Button>
          )}
        </div>
      </div>
    </Form>
  );
}
