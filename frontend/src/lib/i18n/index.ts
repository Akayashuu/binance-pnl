import { addMessages, init, register, getLocaleFromNavigator } from 'svelte-i18n';
import en from './locales/en.json';
import fr from './locales/fr.json';

const STORAGE_KEY = 'binancetracker.locale';

addMessages('en', en);
addMessages('fr', fr);

register('en', () => Promise.resolve(en));
register('fr', () => Promise.resolve(fr));

export function setupI18n() {
	const stored = typeof window !== 'undefined' ? window.localStorage.getItem(STORAGE_KEY) : null;
	const initial = stored ?? getLocaleFromNavigator() ?? 'en';
	init({
		fallbackLocale: 'en',
		initialLocale: initial.startsWith('fr') ? 'fr' : 'en'
	});
}

export function persistLocale(locale: string) {
	if (typeof window !== 'undefined') {
		window.localStorage.setItem(STORAGE_KEY, locale);
	}
}
