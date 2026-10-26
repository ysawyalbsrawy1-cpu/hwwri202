# app.py — v4.4 (Debug Mode)
import asyncio
import json
import re
import time
import uuid
import logging
import os
from typing import Optional

import httpx
from fastapi import FastAPI, Query
from fastapi.responses import JSONResponse

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
log = logging.getLogger("shopify-api")

app = FastAPI(title="Shopify Checker API", version="4.4.0")

UPSTREAM_TIMEOUT = 90
UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
      "(KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

CC_RE = re.compile(r'^(\d{15,16})\|(\d{2})\|(\d{2,4})\|(\d{3,4})$')

def parse_cc(cc: str) -> Optional[dict]:
    m = CC_RE.match((cc or "").strip())
    if not m: return None
    num, mon, yr, cvv = m.groups()
    if len(yr) == 2: yr = "20" + yr
    return {"number": num, "month": mon, "year": yr, "cvv": cvv}

def parse_proxy(px: str) -> Optional[dict]:
    px = (px or "").strip()
    if not px: return None
    if "@" in px:
        auth, hp = px.split("@", 1)
        if ":" not in auth or ":" not in hp: return None
        user, pwd = auth.split(":", 1)
        host, port = hp.split(":", 1)
        return {"host": host, "port": port, "user": user, "pass": pwd}
    parts = px.split(":")
    if len(parts) == 2: return {"host": parts[0], "port": parts[1], "user": None, "pass": None}
    if len(parts) == 4: return {"host": parts[0], "port": parts[1], "user": parts[2], "pass": parts[3]}
    return None

def proxy_url(p: dict) -> str:
    if p.get("user"): return f"http://{p['user']}:{p['pass']}@{p['host']}:{p['port']}"
    return f"http://{p['host']}:{p['port']}"

def normalize_site(site: str) -> str:
    s = (site or "").strip().lower()
    s = s.replace("https://", "").replace("http://", "").rstrip("/")
    return s

@app.get("/shopify")
async def shopify(site: str = Query(...), cc: str = Query(...), proxy: str = Query(...)):
    t0 = time.time()
    card = parse_cc(cc)
    if not card:
        return JSONResponse(status_code=400, content={"status": False, "message": "INVALID_CC_FORMAT", "time_taken": f"{round(time.time()-t0, 2)}s"})
    
    proxy_parsed = parse_proxy(proxy)
    if not proxy_parsed:
        return JSONResponse(status_code=400, content={"status": False, "message": "INVALID_PROXY_FORMAT", "time_taken": f"{round(time.time()-t0, 2)}s"})
    
    site_clean = normalize_site(site)
    base = f"https://{site_clean}"
    
    try:
        transport = httpx.AsyncHTTPTransport(proxy=proxy_url(proxy_parsed), verify=False)
        async with httpx.AsyncClient(transport=transport, follow_redirects=True, headers={"User-Agent": UA}, timeout=30) as client:
            # الخطوة 1: جلب المنتجات
            try:
                r = await client.get(f"{base}/products.json?limit=5", timeout=15)
                if r.status_code != 200:
                    return {"status": False, "message": f"PRODUCTS_ERROR_{r.status_code}", "time_taken": f"{round(time.time()-t0, 2)}s"}
                data = r.json()
            except Exception as e:
                return {"status": False, "message": f"PRODUCTS_FAILED: {str(e)[:50]}", "time_taken": f"{round(time.time()-t0, 2)}s"}
                
            # الخطوة 2: إنشاء السلة
            variant_id = None
            for product in data.get("products", []):
                for v in product.get("variants", []):
                    variant_id = v.get("id")
                    break
                if variant_id: break
                
            if not variant_id:
                return {"status": False, "message": "NO_VARIANT_FOUND", "time_taken": f"{round(time.time()-t0, 2)}s"}

            try:
                await client.post(f"{base}/cart/add.js", data={"id": str(variant_id), "quantity": "1"}, timeout=15)
            except Exception as e:
                return {"status": False, "message": f"CART_FAILED: {str(e)[:50]}", "time_taken": f"{round(time.time()-t0, 2)}s"}

            # الخطوة 3: جلب صفحة الـ Checkout
            try:
                ck = await client.get(f"{base}/checkout", timeout=15)
                if "checkouts" not in str(ck.url):
                    return {"status": False, "message": f"CHECKOUT_URL_FAILED", "time_taken": f"{round(time.time()-t0, 2)}s"}
            except Exception as e:
                return {"status": False, "message": f"CHECKOUT_FAILED: {str(e)[:50]}", "time_taken": f"{round(time.time()-t0, 2)}s"}

            return {"status": False, "message": "REACHED_CHECKOUT_SUCCESSFULLY", "time_taken": f"{round(time.time()-t0, 2)}s", "site": site_clean}
            
    except Exception as e:
        return {"status": False, "message": f"GLOBAL_ERROR: {str(e)[:100]}", "time_taken": f"{round(time.time()-t0, 2)}s"}

@app.get("/health")
async def health(): return {"ok": True, "service": "shopify-checker", "ts": int(time.time())}
