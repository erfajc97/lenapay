import { type Control, Controller, type FieldValues, type Path } from 'react-hook-form';
import { useTranslation } from 'react-i18next';

import { TextField, type TextFieldProps } from './TextField';

/** A TextField bound to react-hook-form; schema messages are i18n keys (M8). */
export function FormField<T extends FieldValues>({ control, name, ...props }: { control: Control<T>; name: Path<T> } & Omit<TextFieldProps, 'value' | 'onChangeText' | 'onBlur' | 'error'>) {
  const { t } = useTranslation();
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <TextField
          {...props}
          value={String(field.value ?? '')}
          onChangeText={field.onChange}
          onBlur={field.onBlur}
          error={fieldState.error?.message ? t(fieldState.error.message) : null}
        />
      )}
    />
  );
}
