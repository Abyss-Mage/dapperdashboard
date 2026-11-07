'use client';
import {useEffect,useState} from 'react';
import {Moon,Sun} from 'lucide-react';

export default function ThemeToggle({className='secondary',compact=false}:{className?:string;compact?:boolean}={}) {
 const [theme,setTheme]=useState<'light'|'dark'>('light');
 const [ready,setReady]=useState(false);
 useEffect(()=>{
  try {setTheme(localStorage.getItem('dapper-theme')==='dark'?'dark':'light')} catch {}
  setReady(true);
  function sync(event:StorageEvent) {
   if(event.key==='dapper-theme'||event.key===null)setTheme(event.newValue==='dark'?'dark':'light');
  }
  window.addEventListener('storage',sync);
  return ()=>window.removeEventListener('storage',sync);
 },[]);
 useEffect(()=>{
  if(!ready)return;
  document.documentElement.dataset.theme=theme;
  try {localStorage.setItem('dapper-theme',theme)} catch {}
 },[theme,ready]);
 return <button type="button" className={className} disabled={!ready} onClick={()=>{const root=document.documentElement;root.classList.add('dd-theme-fade');window.setTimeout(()=>root.classList.remove('dd-theme-fade'),600);setTheme(current=>current==='dark'?'light':'dark')}} aria-label={theme==='dark'?'Switch to light theme':'Switch to dark theme'} title={theme==='dark'?'Switch to light theme':'Switch to dark theme'}>{compact?(theme==='dark'?<Sun size={19} aria-hidden="true"/>:<Moon size={19} aria-hidden="true"/>):(theme==='dark'?'Light mode':'Dark mode')}</button>;
}
