"use client";

import Link from "next/link";
import type { ConnState } from "@/lib/useDevices";
import type { MqttStatus } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

// Explains why there are no devices: backend unreachable, MQTT not set up,
// or the broker connection failing.
export function MqttBanner({ conn, mqtt }: { conn: ConnState; mqtt: MqttStatus | null }) {
  const { t } = useI18n();

  if (conn === "closed") {
    return (
      <div className="banner bad">
        <span>
          <strong>{t("banner.backendUnreachable")}</strong> {t("banner.retrying")}
        </span>
      </div>
    );
  }
  if (!mqtt || mqtt.state === "connected") return null;

  const settings = (
    <Link href="/settings/" className="banner-action">
      {t("banner.mqttSettings")}
    </Link>
  );

  switch (mqtt.state) {
    case "unconfigured":
      return (
        <div className="banner warn">
          <span>
            <strong>{t("banner.notConfigured")}</strong> {t("banner.enterBroker")}
          </span>
          {settings}
        </div>
      );
    case "connecting":
      return (
        <div className="banner info">
          <span>{t("banner.connectingTo", { broker: mqtt.broker ?? "" })}</span>
          {settings}
        </div>
      );
    case "error":
      return (
        <div className="banner bad">
          <span>
            <strong>{t("banner.cannotConnect", { broker: mqtt.broker ?? "" })}</strong>
            {mqtt.error && <span className="banner-detail">{mqtt.error}</span>}
            <span className="banner-detail">{t("banner.retryingBackground")}</span>
          </span>
          {settings}
        </div>
      );
    default:
      return null;
  }
}
