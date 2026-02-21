import { create } from 'zustand';
import { createProjectsSlice } from './projects/projectsSlice';
import type { ProjectsSlice } from './projects/projects.types';

// The unified store type — extend with additional slices (e.g. AuthSlice) as needed
export type StoreState = ProjectsSlice;

export const useStore = create<StoreState>()((...args) => ({
  ...createProjectsSlice(...args),
}));
