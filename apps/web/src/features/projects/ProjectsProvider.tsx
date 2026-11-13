import { createContext, useContext, useState, type ReactNode } from 'react'
import { projectSamples, type Project } from './data'
export type NewProject = Pick<Project, 'name' | 'region' | 'client' | 'budget' | 'description'>
interface ProjectStore { projects: Project[]; create: (input: NewProject) => Project }
const Context = createContext<ProjectStore | null>(null)
export function ProjectsProvider({ children }: { children: ReactNode }) {
  const [projects, setProjects] = useState(projectSamples)
  function create(input: NewProject) {
    const record: Project = { ...input, id: 'local-' + crypto.randomUUID(), date: new Date().toISOString().slice(0, 10), local: true }
    setProjects(previous => [record, ...previous])
    return record
  }
  return <Context.Provider value={{ projects, create }}>{children}</Context.Provider>
}
export function useProjects() {
  const value = useContext(Context)
  if (!value) throw new Error('ProjectsPage requires ProjectsProvider')
  return value
}
