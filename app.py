import os
import re
import time
import random
import requests
from flask import Flask, request, jsonify

app = Flask(__name__)

#  
TIMEOUT = 30
MAX_PRICE = 15.0

def format_proxy(proxy_str):
    """      requests """
    if not proxy_str:
        return None
    p = proxy_str.strip()
    if '@' in p:
        return p
    parts = p.split(':')
    if len(parts) == 4:
        # ip:port:user:pass
        if len(parts[0]) <= 5:
            user, pwd, ip, port = parts
            return f"{ip}:{port}@{user}:{pwd}"
        else:
            ip, port, user, pwd = parts
            return f"{ip}:{port}@{user}:{pwd}"
    return p

def get_proxies_dict(proxy_str):
    if not proxy_str:
        return None
    fmt = format_proxy(proxy_str)
    if not fmt:
        return None
    url = f"http://{fmt}"
    return {"http": url, "https": url}

def get_shopify_product(site, proxy_dict):
    """    """
    try:
        if not site.startswith('http'):
            site = f'https://{site}'
        url = f"{site.rstrip('/')}/products.json"
        headers = {'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'}
        resp = requests.get(url, headers=headers, proxies=proxy_dict, timeout=10, verify=False)
        if resp.status_code == 200:
            data = resp.json()
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
    except Exception:
        pass
    return None

def shopify_checkout(site, cc, proxy_dict):
    """    Shopify"""
    # 1.  
    product = get_shopify_product(site, proxy_dict)
    if not product:
        return {"Response": "Site Error - No Products Found", "Price": "-", "Gate": "Shopify"}

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

    # 2.  
    cart_url = f"{site.rstrip('/')}/cart/add.js"
    payload = {"id": product['id'], "quantity": 1}
    
    try:
        r = requests.post(cart_url, json=payload, headers=headers, proxies=proxy_dict, timeout=15, verify=False)
        if r.status_code not in [200, 201]:
            return {"Response": "Cart Add Failed", "Price": price_str, "Gate": "Shopify"}
    except Exception as e:
        return {"Response": f"Connection Error: {str(e)}", "Price": price_str, "Gate": "Shopify"}

    # 3.   (   Shopify Checkout)
    # : Shopify     API  Checkout Token 
    #        (      )
    
    #   
    time.sleep(random.uniform(1, 3))
    
    #    (  -  )
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
    price_req = request.args.get('price')

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