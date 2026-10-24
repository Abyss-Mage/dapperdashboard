'use client';
import {useEffect, useState} from 'react';
import InvoiceManager from '../../../apps/web/app/workspace/InvoiceManager';
import CustomerAccountManager from '../../../apps/web/app/workspace/CustomerAccountManager';
import ReservedBookingForm from '../../../apps/web/app/workspace/ReservedBookingForm';
import BookingManager from '../../../apps/web/app/workspace/BookingManager';
import BookingOfferManager from '../../../apps/web/app/workspace/BookingOfferManager';
import CustomerAvailabilityBrowser from '../../../apps/web/app/account/CustomerAvailabilityBrowser';
import CustomerBookingRequestList from '../../../apps/web/app/account/CustomerBookingRequestList';
import OperationalJourneyAcceptance from './OperationalJourneyAcceptance';

export default function InvoiceBrowserAcceptance() {
  const [base, setBase] = useState('');
  const [suite,setSuite]=useState('');
  const [actorId,setActorId]=useState(''),[startHint,setStartHint]=useState('');
  const [customerId,setCustomerId]=useState(''),[vehicles,setVehicles]=useState<{id:string;description:string;categoryDescription:string}[]>([]);
  const [initialBookingId, setInitialBookingId] = useState('');
  const [serviceId, setServiceId] = useState('');
  const [requestRevision,setRequestRevision]=useState(0);
  useEffect(() => {
    fetch('/acceptance-context')
      .then(response => response.json())
      .then(body => {
        setBase(body.base);
        setSuite(body.suite);
        setActorId(body.actorId);
        setStartHint(body.startHint);
        setCustomerId(body.customerId);
        setVehicles(body.vehicles ?? []);
        setInitialBookingId(body.initialBookingId ?? '');
        setServiceId(body.serviceId ?? '');
      });
  }, []);
  return <main style={{padding: '1rem', maxWidth: '1100px', margin: 'auto'}}>
    <h1>Isolated browser acceptance</h1>
    <p>Synthetic test records only. Separate disposable PostgreSQL schema; no production application data or provider operations.</p>
    {(suite.startsWith('booking-')||suite==='customer-availability'||suite==='customer-request')&&<p>Synthetic fixture start: {startHint}</p>}
    {base && (
      suite === 'operational-journey' ? (
        <OperationalJourneyAcceptance
          base={base}
          customerId={customerId}
          serviceId={serviceId}
          initialBookingId={initialBookingId}
          vehicles={vehicles}
        />
      ) : (suite==='customer-availability'||suite==='customer-request')?<CustomerAvailabilityBrowser customerId={customerId} vehicles={vehicles} onSubmitted={()=>setRequestRevision(current=>current+1)}/>:suite==='booking-offers'?<BookingOfferManager base={base}/>: (suite==='booking-cancellation'||suite==='booking-reschedule')?<BookingManager base={base} canCreate={false} canManage canCreateWorkOrders={false}/>:suite==='booking-capacity'?<ReservedBookingForm base={base} actorId={actorId}/>:suite==='customer-accounts'?<CustomerAccountManager base={base}/>:<InvoiceManager base={base} canCreate canManage canRecordPayment canIssue />
    )}
    {base&&suite==='customer-request'&&<CustomerBookingRequestList endpoint={`/api/v1/portal/accounts/${customerId}/booking-requests`} revision={requestRevision}/>}
  </main>;
}
