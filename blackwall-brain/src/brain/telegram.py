"""Telegram Bot API sender."""
import logging
import time

import httpx

log = logging.getLogger(__name__)

ATTEMPTS = 3
BACKOFF_S = (2, 4)


def send(token: str, chat_id: str, text: str, client: httpx.Client | None = None,
         sleep=time.sleep) -> bool:
    client = client or httpx.Client(timeout=15)
    payload = {"chat_id": chat_id, "text": text, "parse_mode": "HTML",
               "disable_web_page_preview": True}
    for attempt in range(1, ATTEMPTS + 1):
        try:
            resp = client.post(f"https://api.telegram.org/bot{token}/sendMessage", json=payload)
            if resp.status_code == 200:
                return True
            # Never log the URL: it carries the bot token.
            log.warning("telegram HTTP %d (%d/%d): %s", resp.status_code, attempt, ATTEMPTS,
                        resp.text[:300])
        except httpx.HTTPError as exc:
            log.warning("telegram request failed (%d/%d): %s", attempt, ATTEMPTS,
                        type(exc).__name__)
        if attempt < ATTEMPTS:
            sleep(BACKOFF_S[attempt - 1])
    return False
