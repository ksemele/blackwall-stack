"""Telegram Bot API sender."""
import logging
import time

import httpx

log = logging.getLogger(__name__)

ATTEMPTS = 3
BACKOFF_S = (2, 4)
MAX_RETRY_AFTER_S = 60
MAX_RATE_LIMIT_WAITS = 5


def send(token: str, chat_id: str, text: str, client: httpx.Client | None = None,
         sleep=time.sleep) -> bool:
    client = client or httpx.Client(timeout=15)
    payload = {"chat_id": chat_id, "text": text, "parse_mode": "HTML",
               "disable_web_page_preview": True}
    attempt = waits = 0
    while attempt < ATTEMPTS:
        attempt += 1
        try:
            resp = client.post(f"https://api.telegram.org/bot{token}/sendMessage", json=payload)
        except httpx.HTTPError as exc:
            log.warning("telegram request failed (%d/%d): %s", attempt, ATTEMPTS,
                        type(exc).__name__)
        else:
            if resp.status_code == 200:
                return True
            # Never log the URL: it carries the bot token.
            log.warning("telegram HTTP %d (%d/%d): %s", resp.status_code, attempt, ATTEMPTS,
                        resp.text[:300])
            if resp.status_code == 429 and waits < MAX_RATE_LIMIT_WAITS:
                # Rate limited: wait as told; this is not a failed attempt.
                waits += 1
                sleep(min(_retry_after(resp), MAX_RETRY_AFTER_S))
                attempt -= 1
                continue
            if 400 <= resp.status_code < 500:
                return False   # bad request, token or chat: retrying will not help
        if attempt < ATTEMPTS:
            sleep(BACKOFF_S[attempt - 1])
    return False


def _retry_after(resp: httpx.Response) -> int:
    try:
        return int(resp.json()["parameters"]["retry_after"])
    except (ValueError, KeyError, TypeError):
        return BACKOFF_S[-1]
