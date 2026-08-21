import { useFormContext } from "react-hook-form";
import { useFormConfig } from "./FormContext";

type FormFieldProps = {
  label?: string;
  name: string;
  type?: string;
  placeholder?: string;
  isEditing?: boolean;
  width?: string;
  validateOnChange?: boolean;
};

export function FormField({
  label,
  name,
  type = "text",
  placeholder,
  isEditing: isEditingProp,
  width: widthProp,
  validateOnChange,
}: FormFieldProps) {
  const {
    register,
    watch,
    trigger,
    formState: { errors },
  } = useFormContext();
  const { isEditing: ctxEditing } = useFormConfig();

  const width = widthProp ?? "w-full";
  const isEditing = isEditingProp ?? ctxEditing ?? false;

  const error = errors[name];
  const value = watch(name) ?? "";

  const { onChange: rhfOnChange, ...registerRest } = register(
    name,
    type === "number" ? { valueAsNumber: true } : undefined,
  );

  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label className="text-muted" htmlFor={name}>
          {label}
        </label>
      )}
      {isEditing ? (
        <>
          <input
            className={`focus:shadow-outline ${width} appearance-none rounded border px-3 py-2 leading-tight shadow focus:outline-none`}
            id={name}
            type={type}
            placeholder={placeholder}
            {...registerRest}
            onChange={(e) => {
              rhfOnChange(e);
              if (validateOnChange) trigger(name);
            }}
            onKeyDown={
              type === "number"
                ? (e) => {
                    if (e.key === "e" || e.key === "E") e.preventDefault();
                  }
                : undefined
            }
          />
          {error && <span className="text-berry-500 text-xs">{error.message as string}</span>}
        </>
      ) : (
        <span>{value}</span>
      )}
    </div>
  );
}
