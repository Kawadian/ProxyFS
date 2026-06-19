import * as LabelPrimitive from "@radix-ui/react-label";

export function Label({
  className = "",
  ...props
}: React.ComponentProps<typeof LabelPrimitive.Root>) {
  return <LabelPrimitive.Root className={`form-label ${className}`} {...props} />;
}

export function Input({
  className = "",
  ...props
}: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input className={`form-input ${className}`} {...props} />;
}

export function Select({
  className = "",
  children,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select className={`form-select ${className}`} {...props}>
      {children}
    </select>
  );
}

export function Textarea({
  className = "",
  ...props
}: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={`form-textarea ${className}`} {...props} />;
}

export function FormGroup({
  label,
  htmlFor,
  error,
  children,
}: {
  label: string;
  htmlFor?: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="form-group">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {error && <p className="form-error" role="alert">{error}</p>}
    </div>
  );
}
