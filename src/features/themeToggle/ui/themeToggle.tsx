"use client";
import { useState } from "react";
import { IconButton } from "@/shared/ui";
export function ThemeToggle() { const [dark,setDark]=useState(false); return <IconButton aria-label={dark?"Включить светлую тему":"Включить тёмную тему"} onClick={()=>{const next=!dark;setDark(next);document.documentElement.dataset.theme=next?"dark":"light";}}>{dark?"☀":"◐"}</IconButton>; }
