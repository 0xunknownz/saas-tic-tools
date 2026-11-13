import { createFileRoute } from '@tanstack/react-router'
import { WorkbenchPage } from '../features/workbench/WorkbenchPage'
export const Route = createFileRoute('/')({ component: WorkbenchPage })
