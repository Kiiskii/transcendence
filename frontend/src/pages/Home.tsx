import { useEffect, useState } from "react";

import { useTranslation } from "react-i18next";

import { api } from "../api/client";

export default function Home() {
  const { t } = useTranslation();

  const [status, setStatus] = useState("");

  useEffect(() => {
    api
      .get("/health")
      .then((res) => setStatus(res.data))
      .catch(() => setStatus("Backend not reachable"));
  }, []);

  return (
    <div className="mx-auto max-w-2xl p-8">
      
      <h1 className="text-accent text-3xl font-bold">
      {t("home.title")}
      </h1>

      <p className="text-muted mt-2">
        {t("home.backendStatus")}: {status}
      </p>

    </div>
  );
}
