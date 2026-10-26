"""Synthetic local HTTPS acceptance; credentials/cookies remain in memory."""
import sys,json,ssl,urllib.request,urllib.error,http.cookiejar,uuid,hashlib
from pathlib import Path
from datetime import datetime,timezone
root=Path(__file__).resolve().parents[1]
runtime=root/'.cache/workspace-runtime'
credential=json.load(sys.stdin)
context=ssl.create_default_context(cafile=str(runtime/'tls/certificate.pem'))
jar=http.cookiejar.CookieJar()
client=urllib.request.build_opener(urllib.request.HTTPSHandler(context=context),urllib.request.HTTPCookieProcessor(jar))
origin='https://127.0.0.1:8443'
cases=[]
def request(method,path,data=None,expected=200,request_origin=origin):
    payload=None if data is None else json.dumps(data).encode()
    req=urllib.request.Request(origin+path,data=payload,method=method,headers={'Origin':request_origin,'Content-Type':'application/json'})
    try:
        with client.open(req,timeout=10) as response: status=response.status; body=response.read(); ctype=response.headers.get('Content-Type','')
    except urllib.error.HTTPError as response: status=response.code; body=response.read(); ctype=response.headers.get('Content-Type','')
    if status!=expected: raise RuntimeError('Unexpected status for '+method+' '+path.split('/')[3]+': '+str(status))
    if 'application/json' not in ctype: return body.decode('utf-8')
    return json.loads(body) if body else None
try:
    health=request('GET','/api/v1/health')
    assert health['mode']=='scoped-postgres-foundation' and health['productionReady'] is False
    request('GET','/api/v1/auth/me',expected=401);cases.append('anonymous_denied')
    request('POST','/api/v1/auth/login',{'email':credential['email'],'password':credential['password']},204)
    credential.clear();cases.append('managed_password_login_over_verified_tls')
    assert any(cookie.name=='__Host-dapper_workspace' and cookie.secure for cookie in jar)
    actor=request('GET','/api/v1/auth/me')['principalId']
    portal_accounts=request('GET','/api/v1/portal/accounts')
    assert isinstance(portal_accounts['items'],list) and portal_accounts['limit']==100
    # This operator is deliberately not enrolled as a customer. Staff grants
    # must never silently confer ownership, even in the synthetic runtime.
    assert portal_accounts['items']==[]
    request('GET','/api/v1/portal/accounts/'+str(uuid.uuid4())+'/records',expected=403)
    request('POST','/api/v1/portal/accounts',{'customerId':str(uuid.uuid4())},405)
    cases.append('customer_portal_explicit_membership_no_staff_fallback_or_self_link')
    access=request('GET','/api/v1/auth/access');assert len(access['locations'])==1
    scope=access['locations'][0];base='/api/v1/organizations/'+scope['organizationId']+'/locations/'+scope['locationId']
    scheduling_status=200 if 'scheduling.manage' in scope['actions'] else 403
    request('GET',base+'/scheduling/resources',expected=scheduling_status)
    cases.append('scheduling_resource_access_matches_explicit_grant')
    request('GET',base+'/scheduling/resources/'+str(uuid.uuid4())+'/rules',expected=scheduling_status)
    cases.append('scheduling_rule_access_matches_explicit_management_grant')
    read_status=200 if 'scheduling.read' in scope['actions'] else 403
    request('GET',base+'/scheduling/reservations?from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z',expected=read_status)
    cases.append('reservation_calendar_read_requires_explicit_grant')
    request('GET','/api/v1/organizations/'+str(uuid.uuid4())+'/structure',expected=403);cases.append('foreign_organization_denied')
    main=request('POST',base+'/services',{'name':'Synthetic runtime main','kind':'main'},201)
    addon=request('POST',base+'/services',{'name':'Synthetic runtime add-on','kind':'addon','parentId':main['id']},201)
    edited=request('PUT',base+'/services/'+main['id'],{'name':'Synthetic runtime main revised','kind':'main','version':1})
    customer_for_link=request('GET',base+'/customers')['items'][0]
    linked=request('POST',base+'/jobs',{'customerId':customer_for_link['id'],'serviceId':main['id'],'addonIds':[addon['id']]},201)
    assert linked['serviceId']==main['id'] and linked['serviceName']==edited['name']
    assert linked['addons']==[{'serviceId':addon['id'],'name':addon['name']}]
    request('POST',base+'/jobs',{'customerId':customer_for_link['id'],'serviceId':main['id'],'addonIds':[addon['id'],addon['id']]},422)
    request('POST',base+'/jobs',{'customerId':customer_for_link['id'],'serviceId':main['id'],'addons':[{'name':'Forged'}]},400)
    request('POST',base+'/jobs',{'customerId':customer_for_link['id'],'serviceId':addon['id']},422)
    cases.append('linked_main_service_identity_and_standalone_addon_denial')
    request('PUT',base+'/services/'+main['id'],{'name':'Stale fixture','kind':'main','version':1},409)
    request('POST',base+'/services/'+main['id']+'/archive',{'version':edited['version']},409)
    request('POST',base+'/services/'+addon['id']+'/archive',{'version':addon['version']})
    request('POST',base+'/jobs',{'customerId':customer_for_link['id'],'serviceId':main['id'],'addonIds':[addon['id']]},422)
    retained=next(job for job in request('GET',base+'/jobs')['items'] if job['id']==linked['id'])
    assert retained['addons']==linked['addons']
    cases.append('linked_addon_snapshot_duplicate_forgery_and_archive_denial')
    request('POST',base+'/services/'+main['id']+'/archive',{'version':edited['version']});cases.append('catalogue_revision_parent_and_archive_lifecycle')
    customers=request('GET',base+'/customers')['items'];assert customers
    job=request('POST',base+'/jobs',{'customerId':customers[0]['id'],'serviceName':'Synthetic runtime work order'},201)
    if 'jobs.contribute' not in scope['actions']:
        request('POST',base+'/jobs/'+job['id']+'/contribute',{},403)
    request('POST',base+'/jobs/'+job['id']+'/complete',{'completedBy':'forged'},400)
    request('POST',base+'/jobs/'+job['id']+'/complete',{},403,'https://untrusted.example')
    completed=request('POST',base+'/jobs/'+job['id']+'/complete',{});assert completed['completedBy']==actor
    assert len(completed['contributors'])==1 and completed['contributors'][0]['principalId']==actor
    request('POST',base+'/jobs/'+job['id']+'/complete',{},409);cases.append('job_actor_origin_and_completion_lifecycle')
    org='/api/v1/organizations/'+scope['organizationId'];suffix=uuid.uuid4().hex[:8]
    org_access=next((o for o in access.get('organizations',[]) if o['id']==scope['organizationId']),None)
    if org_access and 'catalogue.manage' in org_access['actions']:
        cent_main=request('POST',org+'/central-services',{'name':'Synthetic central main '+suffix,'kind':'main'},201)
        cent_addon=request('POST',org+'/central-services',{'name':'Synthetic central add-on '+suffix,'kind':'addon','parentId':cent_main['id']},201)
        cent_items=request('GET',org+'/central-services')['items']
        assert any(i['id']==cent_main['id'] for i in cent_items)
        request('POST',org+'/central-services/'+cent_addon['id']+'/archive',{'version':cent_addon['version']})
        request('POST',org+'/central-services/'+cent_main['id']+'/archive',{'version':cent_main['version']})
        cases.append('organization_central_catalogue_lifecycle')
    if 'attendance.record' in scope['actions']:
        active_att=request('GET',base+'/attendance/active')['item']
        if active_att:
            request('POST',base+'/attendance/clock-out',{'notes':'Reset active'})
        clock_in=request('POST',base+'/attendance/clock-in',{'notes':'HTTPS verification shift'},201)
        assert clock_in['status']=='clocked_in'
        request('POST',base+'/attendance/clock-in',{'notes':'Double'},409)
        on_brk=request('POST',base+'/attendance/start-break',{'notes':'Break start'})
        assert on_brk['status']=='on_break'
        end_brk=request('POST',base+'/attendance/end-break',{'notes':'Break end'})
        assert end_brk['status']=='clocked_in'
        clock_out=request('POST',base+'/attendance/clock-out',{'notes':'Shift end'})
        assert clock_out['status']=='clocked_out'
        history=request('GET',base+'/attendance/history')['items']
        assert any(h['id']==clock_in['id'] for h in history)
        events=request('GET',base+'/attendance/'+clock_in['id']+'/events')['items']
        assert len(events)==4
        cases.append('employee_attendance_clock_break_and_history_lifecycle')
    if 'bookings.create' in scope['actions'] and 'bookings.read' in scope['actions']:
        booking_svc=request('POST',base+'/services',{'name':'Runtime Booking Detail '+suffix,'kind':'main'},201)
        booking=request('POST',base+'/bookings',{
            'customerId':customer_for_link['id'],
            'serviceId':booking_svc['id'],
            'channel':'internal',
            'status':'confirmed',
            'bookingTime':'2026-11-20T11:00:00Z',
            'notes':'HTTPS runtime booking verification'
        },201)
        assert booking['status']=='confirmed'
        request('POST',base+'/bookings',{
            'customerId':customer_for_link['id'],'vehicleId':'invalid',
            'serviceId':booking_svc['id'],'channel':'internal',
            'bookingTime':'2026-11-20T11:00:00Z'
        },422)
        if 'vehicles.create' in scope['actions'] and 'customers.create' in scope['actions']:
            vehicle=request('POST',base+'/vehicles',{
                'customerId':customer_for_link['id'],'description':'Synthetic booking acceptance equipment',
                'categoryDescription':'Synthetic test equipment','identifier':'BOOKING-'+uuid.uuid4().hex[:10]
            },201)
            vehicle_booking=request('POST',base+'/bookings',{
                'customerId':customer_for_link['id'],'vehicleId':vehicle['id'],
                'serviceId':booking_svc['id'],'channel':'internal','status':'pending',
                'bookingTime':'2026-11-21T11:00:00Z','notes':'Synthetic owned equipment acceptance'
            },201)
            assert vehicle_booking['vehicleId']==vehicle['id']
            other_owner=request('POST',base+'/customers',{
                'name':'Synthetic other booking owner','email':'booking-'+uuid.uuid4().hex+'@example.invalid','phone':''
            },201)
            request('POST',base+'/bookings',{
                'customerId':other_owner['id'],'vehicleId':vehicle['id'],
                'serviceId':booking_svc['id'],'channel':'internal','bookingTime':'2026-11-21T11:00:00Z'
            },422)
            saved=request('GET',base+'/bookings/'+vehicle_booking['id'])['booking']
            assert saved['vehicleId']==vehicle['id'] and saved['customerId']==customer_for_link['id']
            cases.append('booking_vehicle_customer_ownership_and_persistence')
        b_list=request('GET',base+'/bookings')['items']
        assert any(b['id']==booking['id'] for b in b_list)
        b_detail=request('GET',base+'/bookings/'+booking['id'])
        assert b_detail['booking']['id']==booking['id'] and len(b_detail['events'])>=1
        if 'bookings.manage' in scope['actions']:
            scheduled_b=request('POST',base+'/bookings/'+booking['id']+'/status',{'toStatus':'scheduled','notes':'Bay reserved'})
            assert scheduled_b['status']=='scheduled'
        cases.append('production_booking_lifecycle_and_events')
    if 'bookings.read' in scope['actions']:
        cal_feed=request('GET',base+'/calendar.ics')
        assert 'BEGIN:VCALENDAR' in cal_feed and 'END:VCALENDAR' in cal_feed
        cases.append('apple_calendar_rfc5545_feed_subscription')
    if 'invoices.create' in scope['actions'] and 'invoices.read' in scope['actions']:
        inv=request('POST',base+'/invoices',{
            'customerId':customer_for_link['id'],
            'taxCents':1500,
            'notes':'HTTPS runtime invoice verification',
            'items':[
                {'description':'Precision Paint Correction','quantity':1,'unitPriceCents':12000},
                {'description':'Hydrophobic Wheel Coating','quantity':1,'unitPriceCents':4000}
            ]
        },201)
        assert inv['subtotalCents']==16000 and inv['taxCents']==1500 and inv['totalCents']==17500 and inv['dueCents']==17500
        inv_list=request('GET',base+'/invoices')['items']
        assert any(i['id']==inv['id'] for i in inv_list)
        if 'payments.record' in scope['actions']:
            pmt_res=request('POST',base+'/invoices/'+inv['id']+'/payments',{
                'amountCents':7500,
                'method':'square_card',
                'providerReference':'sq_test_https_1',
                'idempotencyKey':'idem-test-'+suffix,
                'notes':'Initial deposit'
            },201)
            updated_inv=pmt_res['invoice']
            assert updated_inv['paidCents']==7500 and updated_inv['dueCents']==10000 and updated_inv['status']=='partially_paid'
            final_pmt=request('POST',base+'/invoices/'+inv['id']+'/payments',{
                'amountCents':10000,
                'method':'cash',
                'notes':'Final cash balance'
            },201)
            assert final_pmt['invoice']['dueCents']==0 and final_pmt['invoice']['status']=='paid'
        cases.append('production_invoicing_itemization_and_exact_payments')
    if 'sop.read' in scope['actions']:
        sops=request('GET',base+'/sops')['items']
        assert len(sops)>=5 and any(s['title']=='Interior Detail SOP' for s in sops)
        if 'sop.acknowledge' in scope['actions']:
            target_sop=next(s for s in sops if s['title']=='Interior Detail SOP')
            ack=request('POST',base+'/sops/'+target_sop['id']+'/acknowledge',{
                'trainingTrack':'orientation',
                'hash':target_sop['sha256Hash'],
                'notes':'HTTPS runtime orientation test'
            },201)
            assert ack['trainingTrack']=='orientation'
            acks=request('GET',base+'/sops/acknowledgments')['items']
            assert any(a['id']==ack['id'] for a in acks)
        cases.append('sop_compliance_registry_and_training_acknowledgments')
    franchise=request('POST',org+'/franchises',{'name':'Synthetic runtime franchise '+suffix},201)
    request('POST',org+'/locations',{'name':'Synthetic runtime location','franchiseId':franchise['id']},201)
    assert len(request('GET','/api/v1/auth/access')['locations'])==1
    cases.append('franchise_location_creation_without_implicit_operational_grants')
    request('POST','/api/v1/auth/logout',expected=204)
    request('GET','/api/v1/auth/me',expected=401);cases.append('logout_session_revoked')
    result={'verifiedAt':datetime.now(timezone.utc).isoformat(),'mode':'synthetic-development-only','transport':'HTTPS with certificate and hostname validation using explicit local certificate trust','browserVerified':False,'productionReady':False,'cases':cases,'certificateFileSha256':hashlib.sha256((runtime/'tls/certificate.pem').read_bytes()).hexdigest()}
    (root/'output/acceptance/scoped-runtime-verification.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
    print('PASS: '+str(len(cases))+' live synthetic HTTPS acceptance groups; no credentials or cookies written')
except Exception as error:
    print('FAIL: scoped HTTPS acceptance ('+type(error).__name__+'); secret values suppressed',file=sys.stderr)
    sys.exit(1)
