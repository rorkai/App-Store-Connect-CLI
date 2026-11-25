"""Verify an exact asc binary against an isolated, stateful TLS provider.

Usage: python3 scripts/test-catalog.py /absolute/path/to/asc
"""

import copy
import json
import os
import socketserver
import ssl
import subprocess
import sys
import tempfile
import threading
from pathlib import Path
from urllib.parse import urlsplit

binary = Path(sys.argv[1]).resolve()
with tempfile.TemporaryDirectory(dir=Path(__file__).parent, prefix="catalog-fixture-") as temp:
    folder = Path(temp)
    cert, tls_key, asc_key = [folder / name for name in ("cert.pem", "tls.pem", "synthetic.p8")]
    subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-subj", "/CN=local fixture",
                    "-addext", "subjectAltName=DNS:api.appstoreconnect.apple.com", "-keyout", tls_key,
                    "-out", cert, "-days", "1"], capture_output=True, check=True)
    subprocess.run(["openssl", "genpkey", "-algorithm", "EC", "-pkeyopt", "ec_paramgen_curve:P-256",
                    "-out", asc_key], capture_output=True, check=True)
    requests, errors, groups, products, subscriptions = [], [], [], [], []
    fail_product = True
    fail_localization = False
    mutate_after_patch = None
    default_iap_availability = False
    incomplete_collection = None
    versions, localized, availability, territories, schedules = {}, {}, {}, {}, {}
    sub_prices, iap_prices, offers = [], [], []
    def link(kind, ident): return {"data":{"type":kind,"id":ident}}
    def resource(kind, ident, attrs=None, rel=None): return {"type":kind,"id":ident,"attributes":attrs or {},"relationships":rel or {}}
    def collection(rows): return {"data":rows,"links":{"self":"https://api.appstoreconnect.apple.com/fixture"}}
    def missing(kind): return {"errors":[{"status":"404","code":"NOT_FOUND","title":"Missing","detail":f"There is no resource of type '{kind}' with id 'fixture'"}]}

    class Provider(socketserver.StreamRequestHandler):
        def handle(self):
            global fail_product, fail_localization, mutate_after_patch
            try:
                assert self.rfile.readline().decode().split()[:2] == ["CONNECT", "api.appstoreconnect.apple.com:443"]
                while (line := self.rfile.readline()) and line != b"\r\n":
                    pass
                self.request.sendall(b"HTTP/1.1 200 Connection Established\r\n\r\n")
                tls_context = ssl.create_default_context(ssl.Purpose.CLIENT_AUTH)
                tls_context.minimum_version = ssl.TLSVersion.TLSv1_2
                tls_context.load_cert_chain(cert, tls_key)
                with tls_context.wrap_socket(self.request, server_side=True) as tls:
                    with tls.makefile("rb") as stream:
                        method, url, _ = stream.readline().decode().split()
                        path = urlsplit(url).path
                        headers = {}
                        while (line := stream.readline()) and line != b"\r\n":
                            key, value = line.decode().split(":", 1)
                            headers[key.lower()] = value.strip()
                        assert headers.get("authorization", "").startswith("Bearer ")
                        document = json.loads(stream.read(int(headers["content-length"]))) if method in ('POST','PATCH') else {}
                        data = document.get('data')
                    requests.append((method, path))
                    status = "200 OK"
                    if method == 'GET' and path == '/v1/apps/app-local/subscriptionGroups':
                        response = {'data': groups, 'links': {}}
                    elif method == 'GET' and path == '/v1/apps/app-local/inAppPurchasesV2':
                        response = {'data': products, 'links': {'self':'https://api.appstoreconnect.apple.com'+path}}
                    elif method == 'GET' and path == '/v1/subscriptionGroups/group-local/subscriptions':
                        response = {'data':subscriptions,'links':{}}
                    elif method == 'POST' and path == '/v1/subscriptions':
                        assert not subscriptions and data['relationships']['group']['data']['id']=='group-local'
                        subscriptions.append({'type':'subscriptions','id':'sub-local','attributes':data['attributes']})
                        response = {'data':subscriptions[0]}
                    elif method == 'PATCH' and path == '/v1/subscriptions/sub-local':
                        if 'included' in document:
                            sub_prices[:] = [resource('subscriptionPrices','price-'+str(i),p.get('attributes'),p['relationships']) for i,p in enumerate(document['included'])]
                        else:
                            subscriptions[0]['attributes'].update(data['attributes'])
                            if mutate_after_patch:
                                mutate_after_patch();mutate_after_patch=None
                        response = {'data':subscriptions[0]}
                    elif method == 'POST' and path == '/v1/subscriptionGroups':
                        assert not groups and data['relationships']['app']['data']['id'] == 'app-local'
                        groups.append({'type':'subscriptionGroups','id':'group-local','attributes':data['attributes']})
                        response = {'data':groups[-1]}
                    elif method == 'POST' and path == '/v2/inAppPurchases':
                        assert not products and data['relationships']['app']['data']['id'] == 'app-local'
                        if fail_product:
                            status = '409 Conflict'
                            response = {'errors':[{'status':'409','code':'ENTITY_ERROR','title':'Fixture rejection'}]}
                        else:
                            products.append({'type':'inAppPurchases','id':'iap-local','attributes':data['attributes']})
                            response = {'data':products[-1]}
                            if default_iap_availability:
                                kind='inAppPurchaseAvailabilities';ident=kind+'-default'
                                availability[kind]=resource(kind,ident,{'availableInNewTerritories':True})
                                territories[ident]=[resource('territories',t) for t in ('USA','CAN','GBR')]
                    elif method == 'PATCH' and path == '/v2/inAppPurchases/iap-local':
                        products[0]['attributes'].update(data['attributes'])
                        if mutate_after_patch:
                            mutate_after_patch();mutate_after_patch=None
                        response = {'data':products[0]}
                    elif method=='GET' and path.endswith('/versions'):
                        response=collection(versions.get(path.split('/')[-2],[]))
                    elif method=='POST' and path.split('/')[-1] in ('subscriptionGroupVersions','subscriptionVersions','inAppPurchaseVersions'):
                        parent=next(iter(data['relationships'].values()))['data']['id']
                        assert not versions.get(parent)
                        v=resource(data['type'],parent+'-version',{'version':1,'state':'PREPARE_FOR_SUBMISSION'})
                        versions[parent]=[v]
                        response={'data':v}
                    elif method=='GET' and path.endswith('/localizations'):
                        response=collection(localized.get(path.split('/')[-2],[]))
                    elif method=='POST' and path.split('/')[-1] in ('subscriptionGroupLocalizations','subscriptionLocalizations','inAppPurchaseLocalizations'):
                        version=data['relationships']['version']['data']['id']
                        rows=localized.setdefault(version,[])
                        assert not any(r['attributes']['locale']==data['attributes']['locale'] for r in rows)
                        if fail_localization and data['type']=='subscriptionLocalizations':
                            status='409 Conflict'
                            response={'errors':[{'status':'409','code':'ENTITY_ERROR','title':'Fixture localization rejection'}]}
                        else:
                            rows.append(resource(data['type'],version+'-locale',data['attributes']))
                            response={'data':rows[-1]}
                    elif method=='GET' and path.endswith('/pricePoints'):
                        prefix='sub' if '/subscriptions/' in path else 'iap'
                        response=collection([resource('subscriptionPricePoints' if prefix=='sub' else 'inAppPurchasePricePoints',prefix+'-USA',{'customerPrice':'9.99'},{'territory':link('territories','USA')})])
                    elif method=='GET' and '/subscriptionPricePoints/' in path and path.endswith('/equalizations'):
                        response=collection([resource('subscriptionPricePoints','sub-'+t,{'customerPrice':price},{'territory':link('territories',t)}) for t,price in [('USA','9.99'),('CAN','12.99')]])
                    elif method=='GET' and path=='/v1/subscriptions/sub-local/prices':
                        response=collection(copy.deepcopy(sub_prices))
                        for price in response['data']:
                            price['relationships'].setdefault('additionalPriceTiers', {'data': []})
                            price['attributes'].setdefault('planType', 'UPFRONT')
                    elif method=='GET' and path.endswith('/iapPriceSchedule'):
                        if 'iap' not in schedules:status='404 Not Found';response=missing('inAppPurchasePriceSchedules')
                        else:response={'data':schedules['iap']}
                    elif method=='POST' and path=='/v1/inAppPurchasePriceSchedules':
                        assert data['relationships']['inAppPurchase']['data']['id']=='iap-local'
                        schedules['iap']=resource('inAppPurchasePriceSchedules','iap-schedule',rel={'baseTerritory':data['relationships']['baseTerritory']})
                        iap_prices[:] = [resource('inAppPurchasePrices','iap-price-'+str(i),p['attributes'],{'territory':link('territories','USA'),'inAppPurchasePricePoint':p['relationships']['inAppPurchasePricePoint']}) for i,p in enumerate(document['included'])]
                        response={'data':schedules['iap']}
                    elif method=='GET' and path=='/v1/inAppPurchasePriceSchedules/iap-schedule/manualPrices':
                        response=collection(iap_prices)
                    elif method=='GET' and path.endswith('/introductoryOffers'):
                        response=collection(offers)
                    elif method=='POST' and path=='/v1/subscriptionIntroductoryOffers':
                        assert data['relationships']['subscription']['data']['id']=='sub-local'
                        offers.append(resource('subscriptionIntroductoryOffers','offer-local',data['attributes'],data['relationships']))
                        response={'data':offers[-1]}
                    elif method=='DELETE' and path=='/v1/subscriptionIntroductoryOffers/offer-local':
                        offers.clear();response={};status='204 No Content'
                    elif method=='GET' and path.endswith(('/subscriptionAvailability','/inAppPurchaseAvailability')):
                        kind='subscriptionAvailabilities' if path.endswith('/subscriptionAvailability') else 'inAppPurchaseAvailabilities'
                        if kind not in availability:status='404 Not Found';response=missing(kind)
                        else:response={'data':availability[kind]}
                    elif method=='POST' and path.split('/')[-1] in ('subscriptionAvailabilities','inAppPurchaseAvailabilities'):
                        kind=data['type'];ident=kind+'-local'
                        availability[kind]=resource(kind,ident,data['attributes'])
                        territories[ident]=[resource('territories',r['id']) for r in data['relationships']['availableTerritories']['data']]
                        response={'data':availability[kind]}
                    elif method=='GET' and path.endswith('/availableTerritories'):
                        response=collection(territories[path.split('/')[-2]])
                    else:
                        raise AssertionError((method,path))
                    if path == incomplete_collection:
                        response['meta']={'paging':{'total':len(response['data'])+1}}
                    body = json.dumps(response).encode()
                    tls.sendall((f'HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {len(body)}\r\nConnection: close\r\n\r\n').encode()+body)
            except Exception as exc:
                errors.append(repr(exc))

    with socketserver.ThreadingTCPServer(('127.0.0.1',0),Provider) as server:
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            proxy = f'http://127.0.0.1:{server.server_address[1]}'
            env = {'PATH':os.environ['PATH'],'HOME':str(folder),'HTTPS_PROXY':proxy,'HTTP_PROXY':proxy,
                   'SSL_CERT_FILE':str(cert),'GODEBUG':'x509sslcertoverrideplatform=1','ASC_BYPASS_KEYCHAIN':'1',
                   'ASC_TELEMETRY_DISABLED':'1','ASC_CONFIG_PATH':str(folder/'missing.json'),
                   'ASC_KEY_ID':'LOCAL_ONLY','ASC_ISSUER_ID':'LOCAL_ONLY','ASC_PRIVATE_KEY_PATH':str(asc_key)}
            manifest = {'groups':[{'referenceName':'Pro'}], 'inAppPurchases':[
                {'productId':'com.example.lifetime','name':'Lifetime','type':'NON_CONSUMABLE'}]}
            def run(mode, payload=None, output='json', confirm=False):
                requests.clear()
                (folder/'catalog.json').write_text(json.dumps(manifest if payload is None else payload))
                result = subprocess.run([binary,'catalog',mode,'--app','app-local','--file','catalog.json','--output',output]+(['--confirm'] if confirm else []),
                                        cwd=folder,env=env,capture_output=True,text=True,timeout=30)
                assert not errors, errors
                return result
            result = run('apply',output='invalid')
            assert result.returncode==2 and not result.stdout and not requests,(result,requests)
            print('PASS invalid output: zero requests')
            result = run('plan')
            assert result.returncode == 0 and len(requests)==2 and all(m=='GET' for m,p in requests), (result,requests)
            assert [i['action'] for i in json.loads(result.stdout)['items']] == ['create','create'], result
            print('PASS plan: two creates, reads only')
            result = run('apply')
            receipt = json.loads(result.stdout)
            assert result.returncode != 0 and receipt['status']=='partial',result
            assert [i['status'] for i in receipt['items']]==['applied','failed'],receipt
            assert requests == [('GET','/v1/apps/app-local/subscriptionGroups'),('GET','/v1/apps/app-local/inAppPurchasesV2'),
                                ('POST','/v1/subscriptionGroups'),('POST','/v2/inAppPurchases')],requests
            print('PASS partial: group persisted before failed IAP; per-item receipt')
            fail_product = False
            result = run('apply')
            assert result.returncode==0, result
            assert [i['status'] for i in json.loads(result.stdout)['items']]==['unchanged','applied'],result
            assert requests[-1]==('POST','/v2/inAppPurchases') and len(requests)==4,requests
            print('PASS resume: reused group, created only missing IAP')
            result = run('apply')
            assert result.returncode==0 and all(m=='GET' for m,p in requests),result
            assert all(i['action']=='noop' for i in json.loads(result.stdout)['items']),result
            print('PASS repeated apply: zero mutations')
            manifest['inAppPurchases'][0]['name']='Renamed'
            result = run('apply')
            assert result.returncode==0 and requests[-1]==('PATCH','/v2/inAppPurchases/iap-local'),(result,requests)
            assert len(products)==1 and products[0]['attributes']['name']=='Renamed',products
            print('PASS rename: stable productId patches original resource')
            saved_products=copy.deepcopy(products)
            duplicate=copy.deepcopy(products[0]);duplicate['attributes']['productId']='com.example.other'
            products.append(duplicate)
            payload=copy.deepcopy(manifest)
            payload['inAppPurchases'].append({'productId':'com.example.other','name':'Other','type':'NON_CONSUMABLE'})
            result=run('apply',payload)
            assert result.returncode!=0 and not result.stdout and all(m=='GET' for m,p in requests),(result,requests)
            products[:]=saved_products
            print('PASS duplicate provider resource: different product IDs cannot target the same IAP')

            result = run('plan',output='table')
            assert result.returncode==0 and 'Kind' in result.stdout and 'noop' in result.stdout,result
            for bad in [dict(manifest,prices=[]), dict(manifest,inAppPurchases=[{'productId':'p','name':'P','type':'INVALID'}]),
                        dict(manifest,inAppPurchases=manifest['inAppPurchases']*2)]:
                result = run('apply',bad)
                assert result.returncode==2 and not result.stdout and not requests,(result,requests)
            print('PASS preflight: unsupported fields, invalid later item, duplicate identity; zero requests')
            for field, invalid in [('inAppPurchases',{'productId':'pkg!bad','name':'Invalid','type':'CONSUMABLE'}),('subscriptions',{'group':'Pro','productId':'pkg!bad','name':'Invalid','subscriptionPeriod':'ONE_MONTH'})]:
                bad=copy.deepcopy(manifest)
                bad.setdefault(field,[]).append(invalid)
                result=run('apply',bad)
                assert result.returncode==2 and not result.stdout and not requests,(result,requests)
            print('PASS product ID allowlist: late invalid IAP or subscription blocks earlier writes with zero requests')

            bad = dict(manifest,inAppPurchases=[dict(manifest['inAppPurchases'][0],type='CONSUMABLE')])
            bad['groups']=[{'referenceName':'Another'}]
            result = run('apply',bad)
            assert result.returncode!=0 and all(m=='GET' for m,p in requests) and len(groups)==1,(result,requests)
            print('PASS remote preflight: immutable type mismatch blocks earlier group create')
            manifest['subscriptions']=[{'group':'Pro','productId':'com.example.monthly','name':'Monthly','subscriptionPeriod':'ONE_MONTH'}]
            result = run('apply')
            assert result.returncode==0 and requests[-1]==('POST','/v1/subscriptions'),(result,requests)
            assert json.loads(result.stdout)['items'][-1]['id']=='sub-local',result
            result = run('apply')
            assert result.returncode==0 and all(m=='GET' for m,p in requests),(result,requests)
            print('PASS subscriptions: existing group reuse, create and repeated apply without writes')
            manifest['subscriptions'][0]['name']='Monthly Renamed'
            result = run('apply')
            assert result.returncode==0 and requests[-1]==('PATCH','/v1/subscriptions/sub-local'),(result,requests)
            assert subscriptions[0]['attributes']['name']=='Monthly Renamed',subscriptions
            print('PASS subscription rename: stable productId updates the existing subscription')
            for obj in (manifest['groups'][0],manifest['inAppPurchases'][0],manifest['subscriptions'][0]):
                obj['localizations']=[{'locale':'en-US','name':'Pro'}]
            for obj in (manifest['inAppPurchases'][0],manifest['subscriptions'][0]):
                obj['pricing']={'baseTerritory':'USA','customerPrice':'9.99','equalize':True}
                obj['availability']={'territories':['USA','CAN'],'availableInNewTerritories':True}
            manifest['subscriptions'][0]['introductoryOffers']=[{'territory':'USA','offerMode':'FREE_TRIAL','duration':'ONE_WEEK','numberOfPeriods':1}]
            result=run('plan')
            assert result.returncode==0 and all(m=='GET' for m,p in requests),(result,requests)
            fail_localization=True
            result=run('apply')
            receipt=json.loads(result.stdout)
            assert result.returncode!=0 and receipt['status']=='partial',(result,requests)
            failed=[i for i in receipt['items'] if i['status']=='failed']
            assert len(failed)==1 and failed[0]['kind']=='subscriptionLocalization',receipt
            assert any(i['status']=='pending' and i['kind']=='subscriptionPricing' for i in receipt['items']),receipt
            fail_localization=False
            result=run('apply')
            assert result.returncode==0 and not any(m=='POST' and p.endswith('Versions') for m,p in requests),(result,requests)
            print('PASS multi-stage resume: retained products, versions and localizations; resumed failed locale before pricing')
            receipt=json.loads(result.stdout)
            assert all(i['status'] in ('unchanged','applied') for i in receipt['items']),receipt
            assert len(localized)==3 and len(sub_prices)==2 and len(iap_prices)==1 and len(offers)==1 and len(availability)==2,receipt
            writes=[p for m,p in requests if m!='GET']
            assert writes.index('/v2/subscriptionLocalizations') < writes.index('/v1/subscriptions/sub-local') < writes.index('/v1/subscriptionIntroductoryOffers') < writes.index('/v1/subscriptionAvailabilities'),writes
            result=run('apply')
            assert result.returncode==0 and all(m=='GET' for m,p in requests),(result,requests)
            print('PASS full catalog: version localizations, equalized subscription prices, IAP schedule, free trial, availability; second apply zero writes')
            original_offer=copy.deepcopy(offers[0])
            offers[0]['attributes']['startDate']='1999-01-01'
            active_offer=copy.deepcopy(offers[0])
            for dates in [{'endDate':'2000-01-01'},{'startDate':'1999-01-01','endDate':'2000-01-01'}]:
                payload=copy.deepcopy(manifest)
                payload['subscriptions'][0]['introductoryOffers'][0].update(dates)
                result=run('apply',payload,confirm=True)
                assert result.returncode!=0 and not result.stdout and all(m=='GET' for m,p in requests) and offers==[active_offer],(result,requests)
                if 'startDate' not in dates:
                    assert result.returncode==2 and 'endDate' in result.stderr and not requests,(result,requests)
                    continue
                assert 'expired endDate' in result.stderr,result
                offers[0]['attributes']['endDate']=dates['endDate']
                result=run('apply',payload)
                assert result.returncode==0 and all(m=='GET' for m,p in requests),(result,requests)
                offers[:]=[copy.deepcopy(active_offer)]
            offers[:]=[original_offer]
            print('PASS expired offers: active offer retained and matching historical offer unchanged')

            saved_prices=copy.deepcopy(sub_prices)
            sub_prices[0]['relationships']['subscriptionPricePoint']['data']['id']='old-point'
            for incomplete_collection in ('/v1/subscriptions/sub-local/prices','/v1/subscriptionPricePoints/sub-USA/equalizations'):
                result=run('apply')
                assert result.returncode!=0 and 'incomplete collection' in result.stderr and not result.stdout and all(m=='GET' for m,p in requests),(result,requests)
            incomplete_collection=None
            sub_prices[:]=saved_prices
            print('PASS pricing discovery: truncated prices and equalizations block all writes')

            for field,value in [('planType','MONTHLY'),('preserved',True)]:
                sub_prices[0]['attributes'][field]=value
                result=run('apply')
                assert result.returncode!=0 and 'preserved or alternate-plan' in result.stderr and all(m=='GET' for m,p in requests),(result,requests)
                del sub_prices[0]['attributes'][field]

            version=versions['sub-local'][0]
            version['attributes']['state']='IN_REVIEW'
            result=run('apply')
            assert result.returncode==0 and all(m=='GET' for m,p in requests),(result,requests)
            payload=copy.deepcopy(manifest)
            payload['inAppPurchases'][0]['name']='Withheld rename'
            payload['subscriptions'][0]['localizations'][0]['name']='Changed'
            result=run('apply',payload)
            assert result.returncode!=0 and 'IN_REVIEW' in result.stderr and all(m=='GET' for m,p in requests),(result,requests)
            version['attributes']['state']='PREPARE_FOR_SUBMISSION'
            print('PASS in-review version: unchanged locale skips writes and a changed locale blocks apply')

            for field, kind, provider_rows in [('inAppPurchases','inAppPurchaseAvailabilities',products),('subscriptions','subscriptionAvailabilities',subscriptions)]:
                for scenario in ('territories','identity','future setting'):
                    saved_availability=copy.deepcopy(availability)
                    saved_territories=copy.deepcopy(territories)
                    saved_name=provider_rows[0]['attributes']['name']
                    payload=copy.deepcopy(manifest)
                    payload[field][0]['name']='Drift trigger'
                    payload[field][0]['availability']['territories'].append('GBR')
                    def drift():
                        ident=availability[kind]['id']
                        if scenario=='territories':territories[ident].append(resource('territories','FRA'))
                        if scenario=='future setting':availability[kind]['attributes']['availableInNewTerritories']=False
                        if scenario=='identity':
                            availability[kind]['id']=ident+'-replaced'
                            territories[ident+'-replaced']=territories[ident]
                    mutate_after_patch=drift
                    result=run('apply',payload,confirm=True)
                    receipt=json.loads(result.stdout)
                    assert result.returncode!=0 and receipt['status']=='partial' and 'availability changed after planning' in result.stdout,(scenario,result,requests)
                    assert not any(m=='POST' and p.endswith('Availabilities') for m,p in requests),(result,requests)
                    availability.clear();availability.update(saved_availability)
                    territories.clear();territories.update(saved_territories)
                    provider_rows[0]['attributes']['name']=saved_name
                payload=copy.deepcopy(manifest)
                payload[field][0]['availability']['availableInNewTerritories']=False
                result=run('apply',payload)
                assert result.returncode==2 and '--confirm' in result.stderr and all(m=='GET' for m,p in requests),(result,requests)
            print('PASS availability scope: ID, territory set and future-setting drift refuse replacement; true-to-false requires confirm')

            saved_offers=copy.deepcopy(offers)
            for scenario in ('attributes','territory','membership'):
                payload=copy.deepcopy(manifest)
                payload['subscriptions'][0]['name']='Offer drift trigger'
                payload['subscriptions'][0]['introductoryOffers'][0]['duration']='TWO_WEEKS'
                saved_name=subscriptions[0]['attributes']['name']
                def drift():
                    if scenario=='attributes':offers[0]['attributes']['duration']='ONE_MONTH'
                    if scenario=='territory':offers[0]['relationships']['territory']=link('territories','CAN')
                    if scenario=='membership':offers.clear()
                mutate_after_patch=drift
                result=run('apply',payload,confirm=True)
                assert result.returncode!=0 and json.loads(result.stdout)['status']=='partial',(scenario,result,requests)
                assert not any(m=='DELETE' or (m=='POST' and p.endswith('IntroductoryOffers')) for m,p in requests),(result,requests)
                offers[:]=copy.deepcopy(saved_offers)
                subscriptions[0]['attributes']['name']=saved_name
            payload=copy.deepcopy(manifest)
            payload['subscriptions'][0]['name']='Withheld overlap rename'
            extra=copy.deepcopy(offers[0]);extra['id']='offer-future';extra['attributes']['startDate']='2999-01-01'
            offers.append(extra)
            result=run('apply',payload,confirm=True)
            assert result.returncode!=0 and not result.stdout and all(m=='GET' for m,p in requests),(result,requests)
            offers[:]=saved_offers
            print('PASS offer scope: current membership, attributes and territory checked before delete; overlapping different start blocks all writes')

            manifest['subscriptions'][0]['availability']['territories']=['USA']
            manifest['subscriptions'][0]['name']='Withheld rename'
            result=run('apply')
            assert result.returncode==2 and '--confirm' in result.stderr and all(m=='GET' for m,p in requests),(result,requests)
            result=run('apply',confirm=True)
            assert result.returncode==0,(result,requests)
            print('PASS destructive preflight: territory removal blocks all writes without confirm')
            manifest['subscriptions'][0]['introductoryOffers'][0]['duration']='TWO_WEEKS'
            result=run('apply',confirm=True)
            assert result.returncode==0 and sum(m=='DELETE' for m,p in requests)==1 and sum(p=='/v1/subscriptionIntroductoryOffers' and m=='POST' for m,p in requests)==1,(result,requests)
            result=run('apply')
            assert result.returncode==0 and all(m=='GET' for m,p in requests),(result,requests)
            print('PASS changed offer: one confirmed replacement then zero repeat writes')
            for kind, rows, relationship in [('IAP',iap_prices,'inAppPurchasePricePoint'),('subscription',sub_prices,'subscriptionPricePoint')]:
                saved=copy.deepcopy(rows)
                for scenario in ('future','outside territory','ambiguous'):
                    rows[:]=copy.deepcopy(saved)
                    rows[0]['relationships'][relationship]['data']['id']='old-point'
                    extra=copy.deepcopy(rows[0]);extra['id']='extra-price'
                    if scenario=='future':extra['attributes']['startDate']='2999-01-01'
                    if scenario=='outside territory':extra['relationships']['territory']['data']['id']='GBR'
                    rows.append(extra)
                    result=run('apply')
                    assert result.returncode!=0 and all(m=='GET' for m,p in requests),(kind,scenario,result,requests)
                rows[:]=saved
            print('PASS schedule scope: IAP and subscription future, outside-territory and ambiguous prices block all writes')

            groups.clear();products.clear();subscriptions.clear();versions.clear();localized.clear();availability.clear();territories.clear();schedules.clear();sub_prices.clear();iap_prices.clear();offers.clear()
            result=run('plan')
            assert result.returncode==0 and any(i.get('deferred') for i in json.loads(result.stdout)['items']) and all(m=='GET' for m,p in requests),(result,requests)
            result=run('apply')
            assert result.returncode==0,(result,requests)
            result=run('apply')
            assert result.returncode==0 and all(m=='GET' for m,p in requests),(result,requests)
            print('PASS new full catalog: deferred price discovery after creation, all stages, repeat zero writes')
            groups.clear();products.clear();subscriptions.clear();versions.clear();localized.clear();availability.clear();territories.clear();schedules.clear();sub_prices.clear();iap_prices.clear();offers.clear()
            default_iap_availability=True
            result=run('apply',confirm=True)
            assert result.returncode!=0 and 'availability changed after planning' in result.stdout,(result,requests)
            assert not any(m=='POST' and p.endswith('Availabilities') for m,p in requests),(result,requests)
            default_iap_availability=False
            result=run('apply',confirm=True)
            assert result.returncode==0,(result,requests)
            print('PASS new-product default availability: unplanned scope retained; rerun reviews and confirms removal')

        finally:
            server.shutdown()
            thread.join()
