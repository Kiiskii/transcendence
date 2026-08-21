import { z } from "zod";
import { usernameSchema, emailSchema, passwordSchema } from "./common";

export const registerSchema = z.object({
  username: usernameSchema,
  email: emailSchema,
  password: passwordSchema,
});

export type RegisterFormSchema = z.infer<typeof registerSchema>;
