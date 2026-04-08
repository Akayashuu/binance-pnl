import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/**
 * Merge Tailwind class strings with conflict resolution. Re-exported from
 * the shadcn-svelte convention so all UI components share the same helper.
 */
export function cn(...inputs: ClassValue[]): string {
	return twMerge(clsx(inputs));
}
