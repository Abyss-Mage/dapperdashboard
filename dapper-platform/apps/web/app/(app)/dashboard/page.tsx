import OwnerDashboardClient from '../owner/OwnerDashboardClient';

// The main dashboard (owners and administrators). Staff use /workspace.
export default function DashboardPage() {
  return <OwnerDashboardClient />;
}
