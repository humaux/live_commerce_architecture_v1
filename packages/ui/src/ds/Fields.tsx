// Purpose: Accessible field contracts, native controls and responsive form layout; labels/help/errors share one owner.
// Depends on: React, fields.module.css and token widths; no API, storage or translation.
// Used by: gallery forms and later page units. Invariant: caller copy only; errors are associated with the actual control.
"use client";
import { useId, type InputHTMLAttributes, type SelectHTMLAttributes, type TextareaHTMLAttributes, type ReactNode } from "react";
import s from "./fields.module.css";
export type FieldControl = { id: string; "aria-describedby"?: string; "aria-invalid"?: true; required?: boolean; maxLength?: number };
export type FieldProps = {
  label: string; required?: boolean; help?: ReactNode; error?: string; width?: "sm" | "md" | "lg" | "full";
  /** Length and maximum belong to this field, not the section heading. Counter copy is localized by the caller. */
  counter?: { value: string; max: number; format: (length: number, max: number) => string };
  children: (control: FieldControl) => ReactNode;
};
export function Field({ label, required, help, error, width = "full", counter, children }: FieldProps) {
  const id = useId();
  const described = [help ? `${id}-help` : "", error ? `${id}-error` : "", counter ? `${id}-counter` : ""].filter(Boolean).join(" ");
  return <div className={s.field} data-width={width}>
    <div className={s.labelRow}><div className={s.labelTitle}><label htmlFor={id}>{label}</label>{required && <span className={s.required} aria-hidden="true">*</span>}</div>
      {counter && <span id={`${id}-counter`} className={s.counter}>{counter.format(counter.value.length, counter.max)}</span>}</div>
    {children({ id, "aria-describedby": described || undefined, "aria-invalid": error ? true : undefined, required, maxLength: counter?.max })}
    {help && <div id={`${id}-help`} className={s.help}>{help}</div>}
    {error && <div id={`${id}-error`} className={s.error} role="alert">{error}</div>}
  </div>;
}
export function TextInput({ className = "", ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${s.control} ${className}`} />;
}
export function Select({ children, className = "", ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select {...props} className={`${s.control} ${className}`}>{children}</select>;
}
export function Textarea({ className = "", ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`${s.control} ${s.textarea} ${className}`} />;
}
export function NumberInput({ unit, ...props }: Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & { unit: string }) {
  return <div className={s.number}><span aria-hidden="true">{unit}</span><TextInput {...props} type="number" /></div>;
}
export function Checkbox({ label, ...props }: Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & { label: string }) {
  return <label className={s.choice}><input {...props} type="checkbox" />{label}</label>;
}
export function RadioGroup({ label, options, value, onChange, help, disabled = false }: { label: string; options: readonly { value: string; label: string }[]; value: string; onChange: (value: string) => void; help?: string; disabled?: boolean }) {
  const name = useId();
  return <fieldset disabled={disabled} className={s.radio} aria-describedby={help ? `${name}-help` : undefined}><legend>{label}</legend>
    <div className={s.choices}>{options.map(o => <label key={o.value} className={s.choice}><input type="radio" name={name} value={o.value} checked={value === o.value} onChange={() => onChange(o.value)} />{o.label}</label>)}</div>
    {help && <p id={`${name}-help`} className={s.help}>{help}</p>}
  </fieldset>;
}
export function FormGrid({ children }: { children: ReactNode }) { return <div className={s.grid}>{children}</div>; }
