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
export const metadata:Metadata={title:'Dapper Detailing · Operations',description:'Dapper Detailing Pros operations and customer services'};
export default function RootLayout({children}:{children:React.ReactNode}){return <html lang="en"><body><ShopBar />{children}<MotionSystem /><Tooltips /><ConfirmHost /></body></html>}
