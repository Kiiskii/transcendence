import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Form } from "./Form";
import { FormField } from "./FormField";
import { FormTextArea } from "./FormTextArea";
import { addListingSchema, type AddListingFormValues } from "../../schemas/addListing";
import Button from "../objects/Button.tsx";
import { ImageDropzone } from "../objects/ImageDropzone";
import { useImageGallery } from "../../hooks/useImageGallery";

// Mirrors the backend's default MAX_IMAGES_PER_LISTING (see
// backend/internal/config/config.go). The server enforces the real cap on
// upload; this just keeps the UI from letting people queue up more than
// it'll accept.
const MAX_LISTING_IMAGES = 5;

export function AddListingSection() {
  const form = useForm<AddListingFormValues>({
    resolver: zodResolver(addListingSchema),
    mode: "onBlur",
    // TODO: blocked by #109 Add hooks to fetch data from backend (or maybe local frontend e.g. from Profile.tsx?)
    // defaultValues: {
    //   firstname: user.firstname ?? "",
    //   lastname: user.lastname ?? "",
    //   phone_number: user.phone_number ?? "",
    //   location: user.location ?? "",
    // },
  });

  const [photoError, setPhotoError] = useState<string | null>(null);
  const {
    images: photos,
    addFiles: addPhotos,
    removeImage: removePhoto,
  } = useImageGallery({
    maxFiles: MAX_LISTING_IMAGES,
    onError: setPhotoError,
  });

  const handleSubmit = (data: AddListingFormValues) => {
    console.log(data, photos);
    // TODO: blocked by #109 Save to API here. A listing has to exist before
    // images can be uploaded (POST /listings/{id}/images needs the new
    // listing's id), so once CreateListing is wired up: create the listing
    // first, then call useUploadListingImage(listing.id) once per file in
    // `photos` (see src/api/listings.ts).
  };
  // TODO: blocked by #109 Add hooks to save data to backend
  // const { handleSubmit: handleSave, isSubmitting, submitError } = useFormSubmit<AddListingFormValues>(
  // async (data) => {
  //   await api.updateAddListing(data); // whatever your API call looks like
  // }

  return (
    <Form form={form} onSubmit={handleSubmit} isEditing={true}>
      <div className="space-y-4">
        <div className="space-y-2">
          <div className="space-y-1">
            <h2 className="text-foreground text-lg font-bold">Title</h2>
            <FormField name="title" width="max-w-md" validateOnChange />
          </div>
          <div className="space-y-1">
            <h2 className="text-foreground text-lg font-bold">Photos</h2>
            <ImageDropzone
              images={photos}
              onFilesSelected={addPhotos}
              onRemove={removePhoto}
              multiple
              emptyMessage="No photos yet."
              helperText="Drag & drop photos here, or click to browse"
            />
            {photoError && <p className="text-berry-500 text-sm">{photoError}</p>}
          </div>
          <div className="space-y-1">
            <h2 className="text-foreground text-lg font-bold">Description</h2>
            <FormTextArea name="description" />
          </div>
          <div className="space-y-1">
            <h2 className="text-foreground text-lg font-bold">Category</h2>
            <FormField
              name="category"
              width="max-w-md"
              placeholder="e.g. 'Mushroom'"
              validateOnChange
            />
          </div>
          <div className="space-y-1">
            <h2 className="text-foreground text-lg font-bold">Price & Quantity</h2>
            <div className="flex flex-row gap-4">
              <FormField
                name="price"
                label="Price"
                placeholder="*in local currency"
                type="number"
                width="max-w-sm"
                validateOnChange
              />
              <FormField
                name="quantity"
                label="Quantity"
                type="number"
                width="max-w-sm"
                validateOnChange
              />
            </div>
            <FormField
              name="unit"
              label="Unit"
              placeholder="e.g. 'kg' or 'dl'"
              width="max-w-md"
              validateOnChange
            />
          </div>
        </div>
        <Button variant="primary" type="submit" disabled={!form.formState.isValid}>
          {/* TODO: blocked by #109 Insert API here */}
          Save
        </Button>
      </div>
    </Form>
  );
}
