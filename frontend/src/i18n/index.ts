import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import en from "../locales/en/common.json";
import fi from "../locales/fi/common.json";

i18n
  .use(initReactI18next)
  .init({
    resources: {
      en: {
        translation: en,
      },
      fi: {
        translation: fi,
      },
    },

    lng: "en",

    fallbackLng: "en",

    interpolation: {
      escapeValue: false,
    },
  });

export default i18n;