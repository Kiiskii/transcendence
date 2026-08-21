import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Form } from "./Form";
import { FormTextArea } from "./FormTextArea";
import { bioSchema, type BioFormValues } from "../../schemas/common";
import Button from "../objects/Button.tsx";
import { useEffect, useState } from "react";
import { useOwnProfile, useUpdateOwnProfile } from "../../api/profile";
import { isApiError } from "../../api/client";

export function BioSection() {
  const [isEditing, setEditing] = useState(false);
  const { data: profile } = useOwnProfile();
  const update = useUpdateOwnProfile();

  const form = useForm<BioFormValues>({
    resolver: zodResolver(bioSchema),
    mode: "onBlur",
  });
  const {
    formState: { errors, isValid, isSubmitting },
  } = form;

  // Same seeding rule as ContactDetailsSection: fill when the profile lands,
  // never while the user is editing.
  useEffect(() => {
    if (profile && !isEditing) {
      form.reset({ bio: profile.bio ?? "" });
    }
  }, [profile, isEditing, form]);

  const handleSubmit = async (data: BioFormValues) => {
    form.clearErrors("root");
    try {
      // An emptied textarea sends "", which the backend treats as "clear".
      await update.mutateAsync({ bio: data.bio });
      setEditing(false);
    } catch (err) {
      form.setError("root", {
        message: isApiError(err) ? err.message : "Something went wrong. Please try again.",
      });
    }
  };

  return (
    <Form form={form} onSubmit={handleSubmit} isEditing={isEditing}>
      <div className="space-y-2">
        <div className="flex flex-row gap-4">
          <FormTextArea name="bio" validateOnChange />
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
              Edit Text
            </Button>
          )}
        </div>
      </div>
    </Form>
  );
}
