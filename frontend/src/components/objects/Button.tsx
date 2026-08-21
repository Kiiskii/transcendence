type ButtonProps = {
  children: React.ReactNode;
  onClick?: () => void; // Insert event handler here
  disabled?: boolean; // Start true or false
  variant?: "primary" | "secondary" | "tertiary"; // Variants for main buttons, secondaries and tertiaries
  type?: "button" | "submit" | "reset"; // Native button type, defaults to "button"
};

export default function Button({
  children,
  onClick,
  disabled = false,
  variant = "primary",
  type = "button",
}: ButtonProps) {
  // A click on a type="button" has no meaningful default action, so we cancel
  // it. Without this, Chromium runs the click's default action AFTER React has
  // re-rendered: an "enter edit mode" button that swaps itself for a
  // type="submit" Save button mid-click makes the browser fire a synthetic
  // submit via that new button (implicit submission), so the form submits
  // immediately with an empty payload.
  const handleClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    if (type === "button") {
      event.preventDefault();
    }
    onClick?.();
  };

  const baseStyles =
    "px-4 py-2 rounded-full font-medium transition-colors duration-150 " +
    "disabled:opacity-50 disabled:cursor-not-allowed " +
    "focus:outline-none focus:ring-2 focus:ring-offset-2";

  const variantStyles = {
    primary:
      "bg-accent text-accent-contrast hover:bg-accent-hover active:bg-accent-active focus:ring-accent",
    secondary:
      "bg-soft text-soft-contrast border border-line hover:bg-soft-hover active:bg-soft-active focus:ring-accent",
    tertiary: "text-accent-active hover:bg-surface-muter active:bg-surface-soft focus:ring-accent",
  };

  return (
    <button
      type={type}
      onClick={handleClick}
      disabled={disabled}
      className={`${baseStyles} ${variantStyles[variant]}`}
    >
      {children}
    </button>
  );
}
