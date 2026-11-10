import { defineCollection } from 'astro:content';
import { z } from 'astro/zod';
import { docsLoader } from '@astrojs/starlight/loaders';
import { docsSchema } from '@astrojs/starlight/schema';

// Every page states its kind (Diátaxis, plus `overview` for a section's landing
// page) and carries a description. A page without either fails the build.
export const PAGE_KINDS = ['tutorial', 'how-to', 'reference', 'explanation', 'overview'] as const;

export const collections = {
	docs: defineCollection({
		loader: docsLoader(),
		schema: docsSchema({
			extend: z.object({
				description: z.string().min(1),
				kind: z.enum(PAGE_KINDS),
			}),
		}),
	}),
};
