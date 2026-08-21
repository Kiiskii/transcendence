// Zod schemas that we use to validate data closing #47 and for the user management module

import { z } from "zod";

export const usernameSchema = z
  .string()
  .trim()
  .min(1, "Username is required")
  .max(50, "Username must be less than 50 characters")
  .regex(/^\S+$/, "Username cannot contain spaces");
// NOTE: Backend all chars allowed, length 1-50 counted in runes, whitespace
// trimmed at edges, allowed inside, uniqueness case insensitive
export const emailSchema = z
  .string()
  .trim()
  .min(1, "Email is required")
  .max(150, "Email must be less than 150 characters")
  .email("Invalid email address");
// NOTE: Backend caps firstname/lastname at 150 bytes (len() in
// validateProfileInput); the refine below enforces that stricter byte limit
// for multibyte text where the rune-counted max() alone wouldn't catch it.
export const nameSchema = z
  .string()
  .trim()
  .min(1, "Name is required")
  .max(150, "Name must be less than 150 characters")
  .refine((v) => new TextEncoder().encode(v).length <= 150, {
    message: "Name must be less than 150 characters",
  });
export const passwordSchema = z
  .string()
  .trim()
  .min(8, "Password must be at least 8 characters long")
  .max(64, "Password must be less than 64 characters")
  .regex(/^\S+$/, "Password cannot contain spaces");
// NOTE: Backend all chars allowed, length 8-72 counted in bytes, whitespace
// never trimmed
export const phoneSchema = z
  .string()
  .trim()
  .min(1, "Phone number is required")
  .max(15, "Phone number must be less than 15 digits")
  .regex(/^[\d\s()+-]{7,15}$/, "Invalid phone number");
// NOTE: If we want better validation then we could convert to E164 standard
// NOTE: Go rejects titles over 100 bytes (len() in validateListingInput) and
// the DB column is VARCHAR(100) which counts characters. The byte cap is the
// stricter one for multibyte text, so the refine below enforces it too.
export const titleSchema = z
  .string()
  .trim()
  .min(1, "Title is required")
  .max(64, "Title is too long")
  .refine((t) => new TextEncoder().encode(t).length <= 100, {
    message: "Title must be under 100 bytes",
  });
// NOTE: Backend imposes no length limit on description; 1024 here is a UI-only cap
export const descriptionSchema = z.string().trim().max(1024, "Description is too long");
export const categorySchema = z
  .string()
  .trim()
  .min(1, "Category is required")
  .max(50, "Category name is too long");
export const priceSchema = z.number("Price is required").positive("Needs a valid price");
export const quantitySchema = z.int32("Quantity is required").positive("Needs a valid quantity");
export const unitSchema = z.string().trim().min(1, "Unit is required").max(20, "Unit too long");

// NOTE: Backend caps location at 100 bytes (len() in validateProfileInput,
// shared with the addresses table); the refine below enforces that stricter
// byte limit for multibyte text where the rune-counted max() alone wouldn't.
export const locationSchema = z
  .string()
  .trim()
  .min(1, "Location is required")
  .max(100, "Location name is too long")
  .regex(/^[\p{L}\s.'-]+$/u, "Invalid location")
  .refine((v) => new TextEncoder().encode(v).length <= 100, {
    message: "Location name is too long",
  });
// NOTE: If we want real geodata then we will need to link to an API such as OpenMaps

// NOTE: Exports for common schemas that are directly used without wrapper objects
export const bioSchema = z.object({
  bio: z.string().trim().max(1000, "Bio must be less than 1000 characters"),
});
export type BioFormValues = z.infer<typeof bioSchema>;
