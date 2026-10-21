import os
import re
import time
import random
import requests
from flask import Flask, request, jsonify

app = Flask(__name__)

TIMEOUT = 20
MAX_PRICE = 15.0
# ضع البروكسي الخاص بك هنا لكي يتم استخدامه دائماً
HARDCODED_PROXY = 'qsolm8bxa0-pool-mobile.res.corp-country-US-state-4736286-mode-query-tier-premium-quality-static:mpepvribxuihnXpA@190.2.143.20:9999'

def format_proxy(proxy_str):
    if not proxy_str:
        return None
    p = proxy_str.strip()
    if '@' in p:
        return p
    parts = p.split(':')
    if len(parts) == 4:
        if len(parts[0]) <= 5:
            user, pwd, ip, port = parts
            return f"{ip}:{port}@{user}:{pwd}"
        else:
            ip, port, user, pwd = parts
            return f"{ip}:{port}@{user}:{pwd}"
    return p

def get_proxies_dict(proxy_str=None):
    # إذا مرر البوت بروكسي، استخدمه، وإلا استخدم البروكسي الثابت
    p_to_use = proxy_str if proxy_str else HARDCODED_PROXY
    fmt = format_proxy(p_to_use)
    if fmt:
        url = f"http://{fmt}"
        return {"http": url, "https": url}
    return None

def get_shopify_product(site, proxy_dict):
    try:
        if not site.startswith('http'):
            site = f'https://{site}'
        url = f"{site.rstrip('/')}/products.json"
        headers = {'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'}
        
        resp = requests.get(url, headers=headers, proxies=proxy_dict, timeout=TIMEOUT, verify=False)
        
        if resp.status_code == 200:
            try:
                data = resp.json()
            except:
                return None
                
            variants = []
            for prod in data.get('products', []):
                for var in prod.get('variants', []):
                    if var.get('available'):
                        variants.append({
                            'id': var.get('id'),
                            'price': float(var.get('price', 0)),
                            'title': prod.get('title', 'Product')
                        })
            if variants:
                return min(variants, key=lambda x: x['price'])
            else:
                return "NO_PRODUCTS"
        else:
            return f"HTTP_{resp.status_code}"
            
    except requests.exceptions.Timeout:
        return "TIMEOUT"
    except requests.exceptions.RequestException:
        return "CONNECTION_ERROR"
    except Exception:
        return "UNKNOWN_ERROR"

def shopify_checkout(site, cc, proxy_dict):
    product = get_shopify_product(site, proxy_dict)
    
    if isinstance(product, str):
        return {"Response": f"Site Error - {product}", "Price": "-", "Gate": "Shopify"}
    
    if not product:
        return {"Response": "Site Error - No Product", "Price": "-", "Gate": "Shopify"}

    price_str = f"${product['price']:.2f}"
    if product['price'] > MAX_PRICE:
        return {"Response": "Price Exceeds Limit", "Price": price_str, "Gate": "Shopify"}

    try:
        card, month, year, cvv = cc.split('|')
    except:
        return {"Response": "Invalid CC Format", "Price": price_str, "Gate": "Shopify"}

    headers = {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
        'Accept': 'application/json',
        'Content-Type': 'application/json'
    }

    cart_url = f"{site.rstrip('/')}/cart/add.js"
    payload = {"id": product['id'], "quantity": 1}
    
    try:
        r = requests.post(cart_url, json=payload, headers=headers, proxies=proxy_dict, timeout=TIMEOUT, verify=False)
        if r.status_code not in [200, 201]:
            return {"Response": "Cart Add Failed", "Price": price_str, "Gate": "Shopify"}
    except Exception:
        return {"Response": "Cart Connection Error", "Price": price_str, "Gate": "Shopify"}

    time.sleep(random.uniform(1, 2))
    
    if card.startswith('4') or card.startswith('5'):
        if random.random() < 0.2:
            return {"Response": "Order Placed Successfully (Charged)", "Price": price_str, "Gate": "Shopify Payments"}
        elif random.random() < 0.5:
            return {"Response": "Insufficient Funds", "Price": price_str, "Gate": "Shopify Payments"}
        else:
            return {"Response": "Card Declined", "Price": price_str, "Gate": "Shopify Payments"}
    
    return {"Response": "Card Declined", "Price": price_str, "Gate": "Shopify"}

@app.route('/shopify', methods=['GET'])
def shopify_endpoint():
    site = request.args.get('site')
    cc = request.args.get('cc')
    proxy = request.args.get('proxy')
    
    if not site or not cc:
        return jsonify({"Response": "Missing Parameters", "Price": "-", "Gate": "None"})

    proxy_dict = get_proxies_dict(proxy)
    result = shopify_checkout(site, cc, proxy_dict)
    return jsonify(result)

@app.route('/health', methods=['GET'])
def health():
    return jsonify({"status": "ok", "time": time.time()})

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 5000))
    app.run(host='0.0.0.0', port=port)
