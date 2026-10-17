// Section -> Chapter -> Module
// Trimmed copy of the guide's content/ordering.ts (same shape, fewer
// modules) for the fixture modules in this directory.

export type SectionID =
  | 'general'
  | 'bronze'
  | 'silver'
  | 'gold'
  | 'plat'
  | 'adv';

export type Chapter = {
  name: string;
  items: string[];
  description?: string;
};

const MODULE_ORDERING: { [key in SectionID]: Chapter[] } = {
  general: [
    {
      name: 'Language-Specific',
      description:
        'Setup instructions and discussion of language-specific features.',
      items: ['fast-io'],
    },
  ],
  bronze: [
    {
      name: 'Additional',
      items: ['casework'],
    },
    {
      name: 'Conclusion',
      description: 'Congratulations on making it this far!',
      items: ['bronze-conclusion'],
    },
  ],
  silver: [
    {
      name: 'Prefix Sums',
      items: ['prefix-sums'],
    },
    {
      name: 'Sorting & Searching',
      items: [
        'two-pointers',
      ],
    },
  ],
  gold: [
    {
      name: 'Additional Topics',
      description: 'Rarely required.',
      items: ['hashmaps'],
    },
  ],
  plat: [
    {
      name: 'Misc. Topics',
      items: ['matrix-expo'],
    },
  ],
  adv: [
    {
      name: 'Data Structures',
      items: ['count-min'],
    },
  ],
};

export default MODULE_ORDERING;
export const SECTIONS: SectionID[] = Object.keys(
  MODULE_ORDERING
) as SectionID[];
export const SECTION_LABELS: { [key in SectionID]: string } = {
  general: 'General',
  bronze: 'Bronze',
  silver: 'Silver',
  gold: 'Gold',
  plat: 'Platinum',
  adv: 'Advanced',
} as const;

const moduleIDToURLMap: { [key: string]: string } = {};

SECTIONS.forEach(section => {
  MODULE_ORDERING[section].forEach(category => {
    category.items.forEach(moduleID => {
      moduleIDToURLMap[moduleID] = `/${section}/${moduleID}`;
    });
  });
});
