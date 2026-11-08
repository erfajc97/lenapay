import type { Control, Path } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { FormField } from '@/shared/ui/FormField';

interface AddressShape {
  street: string;
  number: string;
  floor: string;
  city: string;
  province: string;
  postalCode: string;
}

/** The contract's Address as form fields (legal address of a company, home of a person). */
export function AddressFields<T extends AddressShape>({ control }: { control: Control<T> }) {
  const { t } = useTranslation();
  const name = (field: keyof AddressShape) => field as Path<T>;
  return (
    <>
      <FormField control={control} name={name('street')} label={t('fields.street')} autoComplete="street-address" textContentType="streetAddressLine1" returnKeyType="next" />
      <View className="flex-row gap-3">
        <View className="flex-1">
          <FormField control={control} name={name('number')} label={t('fields.streetNumber')} keyboardType="number-pad" autoComplete="off" returnKeyType="next" />
        </View>
        <View className="flex-1">
          <FormField control={control} name={name('floor')} label={t('fields.floor')} autoComplete="off" textContentType="streetAddressLine2" returnKeyType="next" />
        </View>
      </View>
      <FormField control={control} name={name('city')} label={t('fields.city')} autoComplete="off" textContentType="addressCity" returnKeyType="next" />
      <View className="flex-row gap-3">
        <View className="flex-1">
          <FormField control={control} name={name('province')} label={t('fields.province')} autoComplete="off" textContentType="addressState" returnKeyType="next" />
        </View>
        <View className="flex-1">
          <FormField control={control} name={name('postalCode')} label={t('fields.postalCode')} autoCapitalize="characters" autoComplete="postal-code" textContentType="postalCode" returnKeyType="done" />
        </View>
      </View>
    </>
  );
}
