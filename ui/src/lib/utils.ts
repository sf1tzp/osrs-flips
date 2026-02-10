import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

/** Parse numeric input with k/m shorthand: "3.5k" → 3500, "2m" → 2000000 */
export function parseNumeric(value: string): number {
	const trimmed = value.trim().toLowerCase();
	if (!trimmed) return 0;
	const match = trimmed.match(/^([0-9]*\.?[0-9]+)\s*([km]?)$/);
	if (!match) return Number(trimmed) || 0;
	const num = parseFloat(match[1]);
	const suffix = match[2];
	if (suffix === 'k') return Math.round(num * 1_000);
	if (suffix === 'm') return Math.round(num * 1_000_000);
	return num;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChild<T> = T extends { child?: any } ? Omit<T, "child"> : T;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChildren<T> = T extends { children?: any } ? Omit<T, "children"> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;
export type WithElementRef<T, U extends HTMLElement = HTMLElement> = T & { ref?: U | null };
