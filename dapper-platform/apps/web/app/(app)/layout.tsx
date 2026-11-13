import type { Metadata } from 'next';
import '@fontsource/manrope/400.css';
import '@fontsource/manrope/500.css';
import '@fontsource/manrope/600.css';
import '@fontsource/manrope/700.css';
import '@fontsource/manrope/800.css';
import './globals.css';
import MotionSystem from './motion/MotionSystem';
import Tooltips from './motion/Tooltips';
import { ConfirmHost } from './confirm';
import ShopBar from './ShopBar';
import TorontoTime from '../TorontoTime';
// The same Dapper icon as the website, on the dashboard, the workspace and the account pages.
export const metadata:Metadata={title:'Dapper Detailing · Operations',description:'Dapper Detailing Pros operations and customer services',icons:{ icon: [{ url: '/favicon.ico', sizes: '48x48' }, { url: '/icon-32.png', type: 'image/png', sizes: '32x32' }, { url: '/icon-192.png', type: 'image/png', sizes: '192x192' }], shortcut: '/favicon.ico', apple: { url: '/apple-touch-icon.png', sizes: '180x180' } }};
export default function RootLayout({children}:{children:React.ReactNode}){return <html lang="en"><body><TorontoTime /><ShopBar />{children}<MotionSystem /><Tooltips /><ConfirmHost /></body></html>}
